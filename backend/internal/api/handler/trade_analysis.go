package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/portfolio"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/internal/tradingday"
)

type TradeAnalysisHandler struct {
	repo       store.PositionRepo
	portfolios store.PortfolioRepo
	analyzer   positionAnalyzer
	log        *zap.Logger

	// T-089 D2 的依賴（trade_plan_status.go）：最新日 K、SR 快照的來源回查、共用的交易日判斷（請求時才取）、時鐘。
	// 任何一項是 nil 時，那一項的狀態是 error／unconfirmed，⛔ 不讓請求失敗。
	candles analysis.LatestCandleReader
	srRefs  srSourceRefReader
	judge   func() tradeDayJudge
	now     func() time.Time
}

type positionAnalyzer interface {
	Analyze(ctx context.Context, symbol string, opts portfolio.AnalyzeOptions) (*portfolio.AnalyzeResult, error)
}

// NewTradeAnalysisHandler：judge 在每次請求時呼叫，取 scheduler 共用的交易日判斷（回 nil 時用 noJudge）。
func NewTradeAnalysisHandler(
	repo store.PositionRepo, portfolios store.PortfolioRepo, analyzer *portfolio.Analyzer,
	candles analysis.LatestCandleReader, srRefs srSourceRefReader, judge func() *tradingday.Judge, log *zap.Logger,
) *TradeAnalysisHandler {
	h := &TradeAnalysisHandler{
		repo: repo, portfolios: portfolios, analyzer: analyzer, log: log,
		candles: candles, srRefs: srRefs, now: time.Now,
	}
	if judge != nil {
		// *tradingday.Judge 是 nil 時回 nil 介面（⛔ 不回「裝著 nil 指標的介面」，那樣會被當成有判斷而 panic）
		h.judge = func() tradeDayJudge {
			if j := judge(); j != nil {
				return j
			}
			return nil
		}
	}
	return h
}

func tradeAnalysisResponse(result *portfolio.AnalyzeResult) gin.H {
	state := ""
	hasPosition := false
	symbol := ""
	if result != nil && result.Analysis != nil {
		symbol = result.Analysis.Symbol
		state = result.Analysis.PositionState
		hasPosition = result.Analysis.Shares > 0
	}
	var positionAnalysis *store.PositionAnalysis
	var sr *store.SRZoneAnalysis
	var zones []store.SRZone
	if result != nil {
		positionAnalysis = result.Analysis
		sr = result.SR
		zones = result.Zones
	}
	return gin.H{
		"context": gin.H{
			"symbol":         symbol,
			"position_state": state,
			"has_position":   hasPosition,
		},
		"analysis":         positionAnalysis,
		"sr_zone_analysis": sr,
		"zones":            zones,
	}
}

func (h *TradeAnalysisHandler) Analyze(c *gin.Context) {
	var body struct {
		Symbol       string `json:"symbol"`
		PortfolioID  uint64 `json:"portfolio_id"`
		Timeframe    string `json:"timeframe"`
		Limit        int    `json:"limit"`
		ForceRefresh bool   `json:"force_refresh"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	body.Symbol = normalizePositionSymbol(body.Symbol)
	if body.Symbol == "" || body.Limit < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid symbol and limit are required"})
		return
	}
	portfolioID := body.PortfolioID
	portfolioID, ok := portfolioIDFromRequest(c, portfolioID)
	if !ok {
		return
	}
	if !requirePortfolioAccess(c, h.portfolios, portfolioID, true) {
		return
	}

	result, err := h.analyzer.Analyze(c.Request.Context(), body.Symbol, portfolio.AnalyzeOptions{
		Timeframe: body.Timeframe, Limit: body.Limit, ForceRefresh: body.ForceRefresh, PortfolioID: portfolioID,
	})
	if err != nil {
		var upstreamErr *analysis.UpstreamStatusError
		if errors.As(err, &upstreamErr) {
			mapScoreZonesError(c, h.log, err)
			return
		}
		serverError(c, h.log, err, "trade analyses: create")
		return
	}
	// T-089 D2：analysis 多 market_date、source_confirmed_1d；頂層多 data_status（只加欄位）
	ctx := c.Request.Context()
	resp := tradeAnalysisResponse(result)
	lookup := sourceLookupOK
	if result != nil && result.Analysis != nil {
		recs, l := h.enrichPlans(ctx, []store.PositionAnalysis{*result.Analysis})
		resp["analysis"] = recs[0]
		lookup = l
	}
	resp["data_status"] = h.buildDataStatus(ctx, portfolioID, body.Symbol, lookup)
	c.JSON(http.StatusCreated, resp)
}

func (h *TradeAnalysisHandler) ListHistory(c *gin.Context) {
	portfolioID, ok := portfolioIDFromQuery(c)
	if !ok {
		return
	}
	if !requirePortfolioAccess(c, h.portfolios, portfolioID, false) {
		return
	}
	symbol := normalizePositionSymbol(c.Param("symbol"))
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	ctx := c.Request.Context()
	rows, err := h.repo.ListAnalyses(ctx, portfolioID, symbol, limit)
	if err != nil {
		serverError(c, h.log, err, "trade analyses: list history")
		return
	}
	// T-089 D2：每筆多 market_date、source_confirmed_1d；頂層多 data_status（⛔ 不收窄：照舊回全部紀錄）
	recs, lookup := h.enrichPlans(ctx, rows)
	c.JSON(http.StatusOK, gin.H{
		"portfolio_id": portfolioID, "analyses": recs, "total": len(recs),
		"data_status": h.buildDataStatus(ctx, portfolioID, symbol, lookup),
	})
}
