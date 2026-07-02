package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/store"
)

type SRZoneHandler struct {
	client *analysis.Client
	repo   store.SRZoneRepo
}

func NewSRZoneHandler(client *analysis.Client, repo store.SRZoneRepo) *SRZoneHandler {
	return &SRZoneHandler{client: client, repo: repo}
}

// POST /api/v1/sr-zones
// Body: { "symbol": "2330", "timeframe": "1d" }
func (h *SRZoneHandler) Create(c *gin.Context) {
	var body struct {
		Symbol    string `json:"symbol"`
		Timeframe string `json:"timeframe"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}
	if body.Timeframe == "" {
		body.Timeframe = "1d"
	}

	result, err := h.client.ScoreZones(c.Request.Context(), body.Symbol, body.Timeframe)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	a, zones, err := result.ToStore()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	id, err := h.repo.Create(c.Request.Context(), a, zones)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	saved, err := h.repo.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	savedZones, err := h.repo.GetZones(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"analysis": saved, "zones": savedZones})
}

// GET /api/v1/sr-zones?symbol=2330&limit=20
func (h *SRZoneHandler) List(c *gin.Context) {
	symbol := c.Query("symbol")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	rows, err := h.repo.List(c.Request.Context(), symbol, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"analyses": rows, "total": len(rows)})
}

// GET /api/v1/sr-zones/:id
func (h *SRZoneHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	a, err := h.repo.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "sr zone analysis not found"})
		return
	}
	zones, err := h.repo.GetZones(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"analysis": a, "zones": zones})
}

// DELETE /api/v1/sr-zones/:id
func (h *SRZoneHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if _, err := h.repo.Get(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "sr zone analysis not found"})
		return
	}
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
