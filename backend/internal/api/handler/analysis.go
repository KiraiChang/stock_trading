package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/store"
)

type AnalysisHandler struct {
	client   *analysis.Client
	verifier *analysis.Verifier
	repo     store.AnalysisRepo
	log      *zap.Logger
}

func NewAnalysisHandler(client *analysis.Client, verifier *analysis.Verifier, repo store.AnalysisRepo, log *zap.Logger) *AnalysisHandler {
	return &AnalysisHandler{client: client, verifier: verifier, repo: repo, log: log}
}

// POST /api/v1/analysis
// Body: { "symbol": "2330", "timeframe": "1d", "limit": 250 }
// limit 省略或為 0 時使用 Python 端的預設值（DEFAULT_FETCH_LIMIT）
func (h *AnalysisHandler) Create(c *gin.Context) {
	var body struct {
		Symbol    string `json:"symbol"`
		Timeframe string `json:"timeframe"`
		Limit     int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}
	if body.Timeframe == "" {
		body.Timeframe = "1d"
	}
	if body.Limit < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be >= 0"})
		return
	}

	result, err := h.client.Analyze(c.Request.Context(), body.Symbol, body.Timeframe, body.Limit)
	if err != nil {
		badGatewayError(c, h.log, err, "analysis: analyze")
		return
	}

	a, levels, err := result.ToStore()
	if err != nil {
		serverError(c, h.log, err, "analysis: convert result to store")
		return
	}

	id, err := h.repo.Create(c.Request.Context(), a, levels)
	if err != nil {
		serverError(c, h.log, err, "analysis: create")
		return
	}

	saved, err := h.repo.Get(c.Request.Context(), id)
	if err != nil {
		serverError(c, h.log, err, "analysis: get saved analysis")
		return
	}
	savedLevels, err := h.repo.GetLevels(c.Request.Context(), id)
	if err != nil {
		serverError(c, h.log, err, "analysis: get saved levels")
		return
	}

	c.JSON(http.StatusCreated, gin.H{"analysis": saved, "levels": savedLevels})
}

// GET /api/v1/analysis?symbol=2330&limit=20
func (h *AnalysisHandler) List(c *gin.Context) {
	symbol := c.Query("symbol")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	rows, err := h.repo.List(c.Request.Context(), symbol, limit)
	if err != nil {
		serverError(c, h.log, err, "analysis: list")
		return
	}
	c.JSON(http.StatusOK, gin.H{"analyses": rows, "total": len(rows)})
}

// GET /api/v1/analysis/:id
func (h *AnalysisHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	a, err := h.repo.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "analysis not found"})
		return
	}
	levels, err := h.repo.GetLevels(c.Request.Context(), id)
	if err != nil {
		serverError(c, h.log, err, "analysis: get levels")
		return
	}
	c.JSON(http.StatusOK, gin.H{"analysis": a, "levels": levels})
}

// POST /api/v1/analysis/:id/verify
func (h *AnalysisHandler) Verify(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	a, levels, err := h.verifier.Verify(c.Request.Context(), id)
	if err != nil {
		serverError(c, h.log, err, "analysis: verify")
		return
	}
	c.JSON(http.StatusOK, gin.H{"analysis": a, "levels": levels})
}

// DELETE /api/v1/analysis/:id
func (h *AnalysisHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if _, err := h.repo.Get(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "analysis not found"})
		return
	}
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		serverError(c, h.log, err, "analysis: delete")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
