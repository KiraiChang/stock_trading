package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

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

// badGatewayError 記錄呼叫上游服務（例如 Python service）失敗的錯誤到 log，
// 同樣不把上游回應細節（可能含內部網址、追蹤訊息）直接回給前端。
func badGatewayError(c *gin.Context, log *zap.Logger, err error, context string) {
	if log != nil {
		log.Error(context, zap.Error(err), zap.String("path", c.Request.URL.Path))
	}
	c.JSON(http.StatusBadGateway, gin.H{"error": "upstream service error"})
}
