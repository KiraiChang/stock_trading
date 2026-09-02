package handler

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/indicator"
)

// jobLookupError 統一處理「查一筆 job」的錯誤：查無資料回 404，其餘（DB 連線中斷、
// 語法錯誤等）回 500。
//
// 先前兩支 job 查詢端點都是「只要 repo 回 error 就一律 404」，DB 掛掉時前端看到的是
// 「任務不存在」而不是伺服器錯誤——使用者會以為任務被清掉了，實際上是資料庫連不上。
func jobLookupError(c *gin.Context, log *zap.Logger, err error, context string) {
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	serverError(c, log, err, context)
}

// serverError 記錄伺服器內部錯誤（DB、內部邏輯等）到 log，並回傳給前端一個
// 不含內部細節的通用訊息——err.Error() 可能包含 DB 連線字串、SQL 片段、
// 內部檔案路徑等不該外洩給客戶端的資訊，這些細節只留在伺服器的 log 裡，
// 前端只需要知道「發生了什麼種類的錯誤」。
func serverError(c *gin.Context, log *zap.Logger, err error, context string) {
	if log != nil {
		log.Error(context, zap.Error(err), zap.String("path", c.Request.URL.Path))
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

// serviceUnavailableError 用於「服務暫時不可用」——目前的來源是指標／訊號寫不進 DB
// （indicator.ErrPersistence）。語意與 serverError 相同：cause 只留在 log，
// 回給客戶端的是固定訊息。
//
// **為什麼不是 500 而是 503**：資料算得出來、只是這一刻存不進去，重試有意義；
// 500 會讓呼叫端以為是程式壞了。前端對狀態碼沒有分支（client.ts 只特判 401），
// 所以這個改動對畫面行為沒有影響，只是語意更誠實。
func serviceUnavailableError(c *gin.Context, log *zap.Logger, err error, context string) {
	if log != nil {
		log.Error(context, zap.Error(err), zap.String("path", c.Request.URL.Path))
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{"error": "service temporarily unavailable"})
}

// indicatorComputeError 是 /indicators/:symbol/compute 與 /signals/:symbol/evaluate
// **共用**的錯誤分流——兩條端點都會走到 indicator.Compute，同一個錯誤在兩邊必須得到
// 同一個狀態碼，否則對外契約自相矛盾（見 docs/issue.md I-102）。
//
// ⚠️ **順序是「先特殊後一般」，不能反**：
//
//  1. ErrPersistence      → 503，cause 只進 log
//  2. ErrInsufficientCandles → 422，訊息要讓使用者知道是資料不夠
//  3. 其餘（含 CandleRepo 讀取失敗）→ serverError 的 500，cause 只進 log
//
// ⛔ **不要退回「其餘一律 422」**：那會把 DB 讀取失敗謊報成「你的輸入有問題」。
// ⛔ **不要回 err.Error()**：它可能帶 DSN、主機位址與 SQL 片段。
func indicatorComputeError(c *gin.Context, log *zap.Logger, err error, context string) {
	switch {
	case errors.Is(err, indicator.ErrPersistence):
		serviceUnavailableError(c, log, err, context)
	case errors.Is(err, indicator.ErrInsufficientCandles):
		if log != nil {
			log.Info(context, zap.Error(err), zap.String("path", c.Request.URL.Path))
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "資料不足，無法計算指標"})
	default:
		serverError(c, log, err, context)
	}
}

// badGatewayError 記錄呼叫上游服務（例如 Python service）失敗的錯誤到 log，
// 同樣不把上游回應細節（可能含內部網址、追蹤訊息）直接回給前端。
func badGatewayError(c *gin.Context, log *zap.Logger, err error, context string) {
	if log != nil {
		log.Error(context, zap.Error(err), zap.String("path", c.Request.URL.Path))
	}
	c.JSON(http.StatusBadGateway, gin.H{"error": "upstream service error"})
}
