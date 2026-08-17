package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// EvaluationUniverseHandler 是評估標的池的 CRUD 入口（T-040 Step 5）。
//
// **這個池不是 watchlist**：它只驅動每日日 K 維護，不進盤中掃描、籌碼同步、signal
// 或 production SR 分析。規格見 docs/evaluation-universe-selection-plan.md
// 的「Step 5 執行計畫書」。
type EvaluationUniverseHandler struct {
	repo store.EvaluationUniverseRepo
	log  *zap.Logger
}

func NewEvaluationUniverseHandler(repo store.EvaluationUniverseRepo, log *zap.Logger) *EvaluationUniverseHandler {
	return &EvaluationUniverseHandler{repo: repo, log: log}
}

// invalidRequest 回 400 並帶上可讀訊息。與 serverError 不同：這裡的訊息是給呼叫端看的
// 驗證失敗原因，不含任何內部細節，所以可以直接回傳。
func invalidRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, gin.H{"error": msg})
}

// evaluationUniverseUpsertItem 是匯入一筆成員的請求形狀。
//
// 刻意**不接受** active：入池/退池是獨立的人工決定，不該被一次重新匯入靜默覆寫
// （repo 的 Upsert 也不動該欄位）。要改用 PATCH。
type evaluationUniverseUpsertItem struct {
	Symbol          string  `json:"symbol"`
	BucketHint      string  `json:"bucket_hint"`
	BucketEdgeLow   float64 `json:"bucket_edge_low"`
	BucketEdgeHigh  float64 `json:"bucket_edge_high"`
	UniverseVersion string  `json:"universe_version"`
	UniverseRole    string  `json:"universe_role"`
	Source          string  `json:"source"`
	Note            string  `json:"note"`
}

type evaluationUniverseUpsertRequest struct {
	Items []evaluationUniverseUpsertItem `json:"items"`
}

// GET /api/v1/evaluation-universe?active=true
func (h *EvaluationUniverseHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	var (
		rows []store.EvaluationUniverseEntry
		err  error
	)
	// 預設回全部（含停用者）——入退池歷史本身是研究紀錄。要熱路徑才明確帶 active=true。
	if c.Query("active") == "true" {
		rows, err = h.repo.ListActive(ctx)
	} else {
		rows, err = h.repo.List(ctx)
	}
	if err != nil {
		serverError(c, h.log, err, "evaluation universe: list")
		return
	}

	activeCount := 0
	for i := range rows {
		if rows[i].Active {
			activeCount++
		}
	}
	buckets := map[string]int{}
	for i := range rows {
		if rows[i].Active {
			buckets[rows[i].BucketHint]++
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"items": rows,
		"total": len(rows),
		// active_count 與 bucket 分佈是判讀選池健康度的第一眼資訊：
		// 三個 bucket 是否都還有足夠樣本，直接決定 T-003 的 sweep 有沒有意義。
		"active_count":   activeCount,
		"active_buckets": buckets,
	})
}

// POST /api/v1/evaluation-universe
// 匯入（或更新）選池成員。以 symbol 為鍵 upsert，**不動 active**。
func (h *EvaluationUniverseHandler) Upsert(c *gin.Context) {
	var req evaluationUniverseUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		invalidRequest(c, "請求格式錯誤")
		return
	}
	if len(req.Items) == 0 {
		invalidRequest(c, "items 不可為空")
		return
	}

	now := time.Now().In(timeutil.TaipeiTZ)
	entries := make([]store.EvaluationUniverseEntry, 0, len(req.Items))
	seen := make(map[string]struct{}, len(req.Items))
	for _, it := range req.Items {
		symbol := strings.TrimSpace(it.Symbol)
		if symbol == "" {
			invalidRequest(c, "symbol 不可為空")
			return
		}
		// 同一次請求裡重複的 symbol 會在同一個 transaction 內互相覆蓋，
		// 最後留下哪一筆取決於順序——那是靜默的資料遺失，直接拒絕。
		if _, dup := seen[symbol]; dup {
			invalidRequest(c, "items 內有重複的 symbol："+symbol)
			return
		}
		seen[symbol] = struct{}{}

		if it.BucketHint == "" {
			invalidRequest(c, "bucket_hint 不可為空："+symbol)
			return
		}
		// 邊界是 bucket_hint 的判定依據，缺了它這筆紀錄就無法回答「用哪組邊界判的」。
		// DB 的 CHECK 也會擋，但在這裡回 400 比讓它變成 500 清楚。
		if it.BucketEdgeLow <= 0 || it.BucketEdgeHigh <= it.BucketEdgeLow {
			invalidRequest(c, "bucket_edge_low/high 不合法："+symbol)
			return
		}
		if it.UniverseVersion == "" {
			invalidRequest(c, "universe_version 不可為空："+symbol)
			return
		}
		if it.Source == "" {
			invalidRequest(c, "source 不可為空："+symbol)
			return
		}
		entries = append(entries, store.EvaluationUniverseEntry{
			Symbol:          symbol,
			BucketHint:      it.BucketHint,
			BucketEdgeLow:   it.BucketEdgeLow,
			BucketEdgeHigh:  it.BucketEdgeHigh,
			UniverseVersion: it.UniverseVersion,
			UniverseRole:    it.UniverseRole,
			// selected_at 由伺服器決定：讓呼叫端指定會讓「何時入池」變成可偽造的欄位，
			// 而它是研究紀錄的一部分。repo 也拒絕零值。
			SelectedAt: now,
			Source:     it.Source,
			Note:       it.Note,
		})
	}

	if err := h.repo.Upsert(c.Request.Context(), entries); err != nil {
		serverError(c, h.log, err, "evaluation universe: upsert")
		return
	}
	c.JSON(http.StatusOK, gin.H{"upserted": len(entries)})
}

type evaluationUniverseActiveRequest struct {
	Active *bool `json:"active"`
}

// PATCH /api/v1/evaluation-universe/:symbol
// 目前只支援切換 active（是否納入每日日 K 維護）。
func (h *EvaluationUniverseHandler) SetActive(c *gin.Context) {
	symbol := strings.TrimSpace(c.Param("symbol"))
	if symbol == "" {
		invalidRequest(c, "symbol 不可為空")
		return
	}
	var req evaluationUniverseActiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		invalidRequest(c, "請求格式錯誤")
		return
	}
	// 指標型別：缺欄位與 false 必須分得開，否則漏帶欄位會被當成「停用」。
	if req.Active == nil {
		invalidRequest(c, "active 為必填")
		return
	}

	ok, err := h.repo.SetActive(c.Request.Context(), symbol, *req.Active)
	if err != nil {
		serverError(c, h.log, err, "evaluation universe: set active")
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "標的不在評估標的池內：" + symbol})
		return
	}
	c.JSON(http.StatusOK, gin.H{"symbol": symbol, "active": *req.Active})
}
