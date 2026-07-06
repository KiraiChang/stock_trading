package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/chip"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// chipInstitutionalLookbackDays 為 /summary 計算法人連續買賣超天數時往回抓的
// 天數（比照 chip.Syncer 的 lookbackDays，涵蓋 20 個交易日的緩衝）。
const chipInstitutionalLookbackDays = 30

type ChipHandler struct {
	institutionalRepo  store.InstitutionalTradeRepo
	marginRepo         store.MarginTradeRepo
	brokerRepo         store.BrokerTradeRepo
	scoreRepo          store.ChipScoreRepo
	candleRepo         store.CandleRepo
	syncJobRepo        store.ChipSyncJobRepo
	syncer             *chip.Syncer
	historyTradingDays int
	log                *zap.Logger
}

func NewChipHandler(
	institutionalRepo store.InstitutionalTradeRepo,
	marginRepo store.MarginTradeRepo,
	brokerRepo store.BrokerTradeRepo,
	scoreRepo store.ChipScoreRepo,
	candleRepo store.CandleRepo,
	syncJobRepo store.ChipSyncJobRepo,
	syncer *chip.Syncer,
	historyTradingDays int,
	log *zap.Logger,
) *ChipHandler {
	return &ChipHandler{
		institutionalRepo:  institutionalRepo,
		marginRepo:         marginRepo,
		brokerRepo:         brokerRepo,
		scoreRepo:          scoreRepo,
		candleRepo:         candleRepo,
		syncJobRepo:        syncJobRepo,
		syncer:             syncer,
		historyTradingDays: historyTradingDays,
		log:                log,
	}
}

func parseChipDate(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", s, timeutil.TaipeiTZ)
}

// GetSummary 查詢單一股票籌碼摘要（GET /api/v1/chips/:symbol/summary?date=）。
// 無 date 時查最新一筆。法人/融資融券/分點區塊各自獨立查詢，任一區塊查無
// 資料時省略該區塊而非整體回錯，呼應設計文件「無資料不應顯示錯誤堆疊」原則。
func (h *ChipHandler) GetSummary(c *gin.Context) {
	symbol := c.Param("symbol")
	ctx := c.Request.Context()

	var score *store.ChipScore
	var err error
	if dateStr := c.Query("date"); dateStr == "" {
		score, err = h.scoreRepo.GetLatest(ctx, symbol)
	} else {
		date, perr := parseChipDate(dateStr)
		if perr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date, expected YYYY-MM-DD"})
			return
		}
		score, err = h.scoreRepo.GetByDate(ctx, symbol, date)
	}
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "chip score not found"})
		return
	}
	date := score.TradeDate

	var reasons []string
	if len(score.Reason) > 0 {
		_ = json.Unmarshal([]byte(score.Reason), &reasons)
	}

	resp := gin.H{
		"symbol":     symbol,
		"date":       date.Format("2006-01-02"),
		"signal":     score.Signal,
		"totalScore": score.TotalScore,
		"reason":     reasons,
	}

	if hist, herr := h.institutionalRepo.GetRange(ctx, symbol, date.AddDate(0, 0, -chipInstitutionalLookbackDays), date); herr == nil && len(hist) > 0 {
		latest := hist[len(hist)-1]
		foreign := make([]int64, len(hist))
		for i, t := range hist {
			foreign[i] = t.ForeignNetBuy
		}
		resp["institutional"] = gin.H{
			"foreignNetBuy":         latest.ForeignNetBuy,
			"investmentTrustNetBuy": latest.InvestmentTrustNetBuy,
			"dealerNetBuy":          latest.DealerNetBuy,
			"consecutiveDays":       chip.CalcConsecutiveNetBuyDays(foreign),
		}
	}

	if margin, merr := h.marginRepo.GetByDate(ctx, symbol, date); merr == nil {
		resp["margin"] = gin.H{
			"marginBalance": margin.MarginBalance,
			"marginChange":  margin.MarginChange,
			"shortBalance":  margin.ShortBalance,
			"shortChange":   margin.ShortChange,
		}
	}

	if brokers, berr := h.brokerRepo.GetByDate(ctx, symbol, date); berr == nil && len(brokers) > 0 {
		topBuy, _ := chip.CalcTopNNetBuy(brokers, 10)
		var dailyVolume int64
		if candles, cerr := h.candleRepo.GetRange(ctx, symbol, "1d", date, date); cerr == nil && len(candles) > 0 {
			dailyVolume = candles[0].Volume
		}
		resp["broker"] = gin.H{
			"topNetBuy":     topBuy,
			"concentration": chip.CalcConcentration(topBuy, dailyVolume),
		}
	}

	c.JSON(http.StatusOK, resp)
}

// GetScores 查詢歷史籌碼分數（GET /api/v1/chips/:symbol/scores?from=&to=）。
func (h *ChipHandler) GetScores(c *gin.Context) {
	symbol := c.Param("symbol")
	fromStr, toStr := c.Query("from"), c.Query("to")
	if fromStr == "" || toStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from and to are required (YYYY-MM-DD)"})
		return
	}
	from, err := parseChipDate(fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from date"})
		return
	}
	to, err := parseChipDate(toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to date"})
		return
	}

	scores, err := h.scoreRepo.GetRange(c.Request.Context(), symbol, from, to)
	if err != nil {
		serverError(c, h.log, err, "chip: get scores")
		return
	}
	c.JSON(http.StatusOK, gin.H{"symbol": symbol, "scores": scores})
}

// GetBrokers 查詢券商分點買賣超排行（GET /api/v1/chips/:symbol/brokers?date=&limit=）。
func (h *ChipHandler) GetBrokers(c *gin.Context) {
	symbol := c.Param("symbol")
	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date is required (YYYY-MM-DD)"})
		return
	}
	date, err := parseChipDate(dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	rows, err := h.brokerRepo.GetByDate(c.Request.Context(), symbol, date)
	if err != nil {
		serverError(c, h.log, err, "chip: get brokers")
		return
	}

	// rows 已依 net_buy DESC 排序：前段為買超排行，反轉後前段為賣超排行。
	// 用 make 而非直接切片 rows，確保 rows 為空時回傳 []（JSON "[]"）而非
	// null，前端不必額外處理兩種空值形狀。
	topBuy := make([]store.BrokerTrade, len(rows))
	copy(topBuy, rows)
	if len(topBuy) > limit {
		topBuy = topBuy[:limit]
	}
	topSell := make([]store.BrokerTrade, len(rows))
	copy(topSell, rows)
	for i, j := 0, len(topSell)-1; i < j; i, j = i+1, j-1 {
		topSell[i], topSell[j] = topSell[j], topSell[i]
	}
	if len(topSell) > limit {
		topSell = topSell[:limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"symbol":  symbol,
		"date":    dateStr,
		"topBuy":  topBuy,
		"topSell": topSell,
	})
}

type chipSyncRequest struct {
	Mode      string   `json:"mode"`
	Symbols   []string `json:"symbols"`
	From      string   `json:"from"`
	To        string   `json:"to"`
	DataTypes []string `json:"dataTypes"`
	Force     bool     `json:"force"`
}

// Sync 手動同步籌碼資料（POST /api/v1/chips/sync），mode 為 manual 或
// backfill 皆共用此端點。立即建立 chip_sync_jobs 紀錄後背景執行，回傳
// job_id 供輪詢（比照 backtest.Manager.Submit 的非同步模式，因為籌碼同步
// 可能是長區間、多檔股票的長任務）。force 目前只接受、暫不做特殊處理
// （Phase 1 一律全部重抓，upsert 天生冪等），保留欄位是為了與設計文件 API
// 形狀一致，待有效能考量再實作 skip 已存在資料的邏輯。
func (h *ChipHandler) Sync(c *gin.Context) {
	var req chipSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Symbols) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbols is required"})
		return
	}
	if req.Mode == "" {
		req.Mode = "manual"
	}
	if req.Mode != "manual" && req.Mode != "backfill" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mode must be manual or backfill"})
		return
	}

	to := timeutil.TodayTaipei()
	if req.To != "" {
		var err error
		to, err = parseChipDate(req.To)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to date"})
			return
		}
	}

	var from time.Time
	if req.From != "" {
		var err error
		from, err = parseChipDate(req.From)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from date"})
			return
		}
	} else if req.Mode == "backfill" {
		from = to.AddDate(0, 0, -h.historyTradingDays)
	} else {
		from = to
	}

	symbolsJSON, _ := json.Marshal(req.Symbols)
	dataTypesJSON, _ := json.Marshal(req.DataTypes)

	job := &store.ChipSyncJob{
		JobID:        newChipSyncJobID(),
		Mode:         req.Mode,
		Symbols:      string(symbolsJSON),
		DataTypes:    string(dataTypesJSON),
		FromDate:     from.Format("2006-01-02"),
		ToDate:       to.Format("2006-01-02"),
		Force:        req.Force,
		Status:       "pending",
		SymbolsTotal: len(req.Symbols),
	}
	if err := h.syncJobRepo.Create(c.Request.Context(), job); err != nil {
		serverError(c, h.log, err, "chip: create sync job")
		return
	}

	go h.runSync(job.JobID, req.Symbols, from, to, req.DataTypes)

	c.JSON(http.StatusAccepted, gin.H{"job": job})
}

func (h *ChipHandler) runSync(jobID string, symbols []string, from, to time.Time, dataTypes []string) {
	ctx := context.Background()
	done, failed := 0, 0
	failures := make([]map[string]string, 0)

	h.syncer.SyncRange(ctx, symbols, from, to, dataTypes, func(symbol string, err error) {
		done++
		if err != nil {
			failed++
			failures = append(failures, map[string]string{"symbol": symbol, "error": err.Error()})
		}
		failuresJSON, _ := json.Marshal(failures)
		if uerr := h.syncJobRepo.UpdateProgress(ctx, jobID, done, failed, store.RawJSON(failuresJSON)); uerr != nil {
			h.log.Warn("chip sync: update progress failed", zap.String("job_id", jobID), zap.Error(uerr))
		}
	})

	status, errMsg := "done", ""
	switch {
	case failed > 0 && failed >= len(symbols):
		status, errMsg = "failed", "all symbols failed"
	case failed > 0:
		status, errMsg = "partial", "some symbols failed"
	}
	if err := h.syncJobRepo.Finish(ctx, jobID, status, errMsg); err != nil {
		h.log.Warn("chip sync: finish job failed", zap.String("job_id", jobID), zap.Error(err))
	}
}

// GetSyncJob 查詢 manual/backfill 同步任務進度（GET /api/v1/chips/sync/:job_id）。
func (h *ChipHandler) GetSyncJob(c *gin.Context) {
	jobID := c.Param("job_id")
	job, err := h.syncJobRepo.GetByJobID(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"job": job})
}

func newChipSyncJobID() string {
	return fmt.Sprintf("chip_%s", time.Now().UTC().Format("20060102_150405_000"))
}
