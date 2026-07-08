package handler

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/portfolio"
	"github.com/trading/backend/internal/store"
)

type HoldingHandler struct {
	repo     store.HoldingRepo
	analyzer *portfolio.Analyzer
	log      *zap.Logger
}

func NewHoldingHandler(repo store.HoldingRepo, analyzer *portfolio.Analyzer, log *zap.Logger) *HoldingHandler {
	return &HoldingHandler{repo: repo, analyzer: analyzer, log: log}
}

type holdingRequest struct {
	Symbol    string  `json:"symbol"`
	Shares    float64 `json:"shares"`
	CostPrice float64 `json:"cost_price"`
	Note      string  `json:"note"`
}

func (h *HoldingHandler) List(c *gin.Context) {
	rows, err := h.repo.List(c.Request.Context())
	if err != nil {
		serverError(c, h.log, err, "holdings: list")
		return
	}
	c.JSON(http.StatusOK, gin.H{"holdings": rows, "total": len(rows)})
}

func (h *HoldingHandler) Create(c *gin.Context) {
	req, ok := bindHoldingRequest(c)
	if !ok {
		return
	}
	id, err := h.repo.Create(c.Request.Context(), &store.Holding{
		Symbol: req.Symbol, Shares: req.Shares, CostPrice: req.CostPrice, Note: req.Note,
	})
	if err != nil {
		serverError(c, h.log, err, "holdings: create")
		return
	}
	row, err := h.repo.Get(c.Request.Context(), id)
	if err != nil {
		serverError(c, h.log, err, "holdings: get created")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"holding": row})
}

func (h *HoldingHandler) Update(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if _, err := h.repo.Get(c.Request.Context(), id); errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "holding not found"})
		return
	} else if err != nil {
		serverError(c, h.log, err, "holdings: get before update")
		return
	}
	req, ok := bindHoldingRequest(c)
	if !ok {
		return
	}
	if err := h.repo.Update(c.Request.Context(), &store.Holding{
		ID: id, Symbol: req.Symbol, Shares: req.Shares, CostPrice: req.CostPrice, Note: req.Note,
	}); err != nil {
		serverError(c, h.log, err, "holdings: update")
		return
	}
	row, err := h.repo.Get(c.Request.Context(), id)
	if err != nil {
		serverError(c, h.log, err, "holdings: get updated")
		return
	}
	c.JSON(http.StatusOK, gin.H{"holding": row})
}

func (h *HoldingHandler) Delete(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if _, err := h.repo.Get(c.Request.Context(), id); errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "holding not found"})
		return
	} else if err != nil {
		serverError(c, h.log, err, "holdings: get before delete")
		return
	}
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		serverError(c, h.log, err, "holdings: delete")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *HoldingHandler) Analyze(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if _, err := h.repo.Get(c.Request.Context(), id); errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "holding not found"})
		return
	} else if err != nil {
		serverError(c, h.log, err, "holdings: get before analyze")
		return
	}
	var body struct {
		Timeframe string `json:"timeframe"`
		Limit     int    `json:"limit"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Limit < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be >= 0"})
		return
	}
	result, err := h.analyzer.Analyze(c.Request.Context(), id, portfolio.AnalyzeOptions{
		Timeframe: body.Timeframe,
		Limit:     body.Limit,
	})
	if err != nil {
		var upstreamErr *analysis.UpstreamStatusError
		if errors.As(err, &upstreamErr) {
			mapScoreZonesError(c, h.log, err)
			return
		}
		serverError(c, h.log, err, "holdings: analyze")
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"analysis":         result.Analysis,
		"sr_zone_analysis": result.SR,
		"zones":            result.Zones,
	})
}

func (h *HoldingHandler) ListAnalyses(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if _, err := h.repo.Get(c.Request.Context(), id); errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "holding not found"})
		return
	} else if err != nil {
		serverError(c, h.log, err, "holdings: get before list analyses")
		return
	}
	rows, err := h.repo.ListAnalyses(c.Request.Context(), id, limit)
	if err != nil {
		serverError(c, h.log, err, "holdings: list analyses")
		return
	}
	c.JSON(http.StatusOK, gin.H{"analyses": rows, "total": len(rows)})
}

func (h *HoldingHandler) GetAnalysis(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	row, err := h.repo.GetAnalysis(c.Request.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "holding analysis not found"})
		return
	} else if err != nil {
		serverError(c, h.log, err, "holdings: get analysis")
		return
	}
	c.JSON(http.StatusOK, gin.H{"analysis": row})
}

func (h *HoldingHandler) DeleteAnalysis(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if _, err := h.repo.GetAnalysis(c.Request.Context(), id); errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "holding analysis not found"})
		return
	} else if err != nil {
		serverError(c, h.log, err, "holdings: get analysis before delete")
		return
	}
	if err := h.repo.DeleteAnalysis(c.Request.Context(), id); err != nil {
		serverError(c, h.log, err, "holdings: delete analysis")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func bindHoldingRequest(c *gin.Context) (holdingRequest, bool) {
	var req holdingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return req, false
	}
	req.Symbol = strings.ToUpper(strings.TrimSpace(req.Symbol))
	req.Note = strings.TrimSpace(req.Note)
	if req.Symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return req, false
	}
	if req.Shares <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "shares must be > 0"})
		return req, false
	}
	if req.CostPrice <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cost_price must be > 0"})
		return req, false
	}
	return req, true
}

func parseUintParam(c *gin.Context, name string) (uint64, error) {
	return strconv.ParseUint(c.Param(name), 10, 64)
}
