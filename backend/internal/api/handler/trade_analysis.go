package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/portfolio"
	"github.com/trading/backend/internal/store"
)

type TradeAnalysisHandler struct {
	repo       store.PositionRepo
	portfolios store.PortfolioRepo
	analyzer   positionAnalyzer
	log        *zap.Logger
}

type positionAnalyzer interface {
	Analyze(ctx context.Context, symbol string, opts portfolio.AnalyzeOptions) (*portfolio.AnalyzeResult, error)
}

func NewTradeAnalysisHandler(repo store.PositionRepo, portfolios store.PortfolioRepo, analyzer *portfolio.Analyzer, log *zap.Logger) *TradeAnalysisHandler {
	return &TradeAnalysisHandler{repo: repo, portfolios: portfolios, analyzer: analyzer, log: log}
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
	c.JSON(http.StatusCreated, tradeAnalysisResponse(result))
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
	rows, err := h.repo.ListAnalyses(c.Request.Context(), portfolioID, symbol, limit)
	if err != nil {
		serverError(c, h.log, err, "trade analyses: list history")
		return
	}
	c.JSON(http.StatusOK, gin.H{"portfolio_id": portfolioID, "analyses": rows, "total": len(rows)})
}
