package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

type StockSymbolHandler struct {
	repo store.StockSymbolRepo
	log  *zap.Logger
}

func NewStockSymbolHandler(repo store.StockSymbolRepo, log *zap.Logger) *StockSymbolHandler {
	return &StockSymbolHandler{repo: repo, log: log}
}

// Candidates 產生研究用的候選標的清單（GET /api/v1/stock-symbols/candidates）。
//
// 用途是 todo.md T-040 的擴標的池：Step 1 要「全部 ETF ＋ 各產業分層抽樣約 300 檔股票」，
// 手工湊 650 個代號不切實際。回傳的 symbols 可直接餵給 POST /api/v1/market/backfill。
//
// Query：
//
//	security_type=股票,ETF   逗號分隔。**留空預設 `股票,ETF`**，見下方說明
//	industry=半導體業,航運業  逗號分隔，空 = 不限
//	listed_years=5           只留上市滿 N 年的（listed_date 為 NULL 者一律排除）。0 或留空 = 不限
//	per_industry=9           每個產業最多幾檔（在該產業代號區間內等距取樣；半導體業有 201 檔，不設限會被它主導）
//	limit=650                總筆數上限，預設 3000、上限 5000
//	include_delisted=true    預設 false
//
// **security_type 為什麼有預設值**：`stock_symbols` 存的是完整的 TWSE ISIN 主檔，
// 實測 43,061 筆上市資料裡有 40,658 筆是認購（售）權證——佔 94%，而且代號排序在股票之前。
// 不給預設值的話，一個沒帶參數的請求會回傳「ETF ＋ 權證」而一檔股票都沒有，
// 且這份清單被設計成可直接餵給 `POST /market/backfill`（無筆數上限、5 req/min），
// 等於把數小時的 FinMind 配額花在沒有 K 線的商品上。要權證請明確指定。
func (h *StockSymbolHandler) Candidates(c *gin.Context) {
	securityTypes := splitCSVParam(c.Query("security_type"))
	if len(securityTypes) == 0 {
		securityTypes = defaultCandidateSecurityTypes
	}
	opts := store.StockSymbolCandidateOptions{
		SecurityTypes: securityTypes,
		Industries:    splitCSVParam(c.Query("industry")),
	}

	if raw := strings.TrimSpace(c.Query("listed_years")); raw != "" {
		years, err := strconv.Atoi(raw)
		if err != nil || years < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "listed_years must be a non-negative integer"})
			return
		}
		// years == 0 視為「不限制」而不是「上市日 <= 現在」。後者會連帶啟用
		// `listed_date IS NOT NULL` 而靜靜濾掉解析失敗的列（parseListedDate 對格式錯誤
		// 只記 warning 不中斷），而前端數字輸入框的預設值很容易就是 0。
		if years > 0 {
			opts.ListedBefore = time.Now().In(timeutil.TaipeiTZ).AddDate(-years, 0, 0)
		}
	}
	if raw := strings.TrimSpace(c.Query("per_industry")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "per_industry must be a non-negative integer"})
			return
		}
		// 0 視為「不限制」，理由同上面的 listed_years：數字輸入框被清空或歸零時會送 0，
		// 那應該回傳不設限的清單，而不是整個請求 400。
		opts.PerIndustryLimit = n
	}
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a positive integer"})
			return
		}
		opts.Limit = n
	}
	if raw := strings.TrimSpace(c.Query("include_delisted")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "include_delisted must be true or false"})
			return
		}
		opts.IncludeDelisted = parsed
	}

	result, err := h.repo.ListCandidates(c.Request.Context(), opts)
	if err != nil {
		serverError(c, h.log, err, "stock symbols: candidates")
		return
	}

	// symbols 是給 market/backfill 直接用的扁平清單；rows 保留完整欄位供人工核對產業分佈。
	symbols := make([]string, 0, len(result.Symbols))
	byIndustry := map[string]int{}
	for _, row := range result.Symbols {
		symbols = append(symbols, row.Symbol)
		byIndustry[row.Industry]++
	}
	c.JSON(http.StatusOK, gin.H{
		"count":       len(result.Symbols),
		"symbols":     symbols,
		"by_industry": byIndustry,
		"rows":        result.Symbols,
		// truncated=true 代表還有更多符合條件的標的被 limit 砍掉，而截斷是依代號順序，
		// 會整批砍掉高代號的產業——呼叫端要知道這份清單不完整。
		"truncated": result.Truncated,
	})
}

// 研究母體預設只看股票與 ETF；權證等其他 ISIN 類別要明確指定才會出現。
var defaultCandidateSecurityTypes = []string{"股票", "ETF"}

func splitCSVParam(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (h *StockSymbolHandler) Search(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	onlyListed := true
	if raw := strings.TrimSpace(c.Query("listed")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "listed must be true or false"})
			return
		}
		onlyListed = parsed
	}

	rows, err := h.repo.Search(c.Request.Context(), store.StockSymbolSearchOptions{
		Query:        c.Query("q"),
		OnlyListed:   onlyListed,
		SecurityType: c.Query("security_type"),
		Limit:        limit,
	})
	if err != nil {
		serverError(c, h.log, err, "stock symbols: search")
		return
	}
	c.JSON(http.StatusOK, gin.H{"symbols": rows})
}
