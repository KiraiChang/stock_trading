package handler

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/portfolio"
	"github.com/trading/backend/internal/store"
)

type PositionHandler struct {
	repo     store.PositionRepo
	analyzer *portfolio.Analyzer
	log      *zap.Logger
}

func NewPositionHandler(repo store.PositionRepo, analyzer *portfolio.Analyzer, log *zap.Logger) *PositionHandler {
	return &PositionHandler{repo: repo, analyzer: analyzer, log: log}
}

func normalizePositionSymbol(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}

func (h *PositionHandler) List(c *gin.Context) {
	rows, err := h.repo.List(c.Request.Context())
	if err != nil {
		serverError(c, h.log, err, "positions: list")
		return
	}
	c.JSON(http.StatusOK, gin.H{"positions": rows, "total": len(rows)})
}

func (h *PositionHandler) Get(c *gin.Context) {
	symbol := normalizePositionSymbol(c.Param("symbol"))
	row, err := h.repo.Get(c.Request.Context(), symbol)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusOK, gin.H{"position": gin.H{
			"symbol": symbol, "shares": 0, "avg_cost": 0, "realized_pnl": 0, "version": 0,
		}})
		return
	}
	if err != nil {
		serverError(c, h.log, err, "positions: get")
		return
	}
	c.JSON(http.StatusOK, gin.H{"position": row})
}

type positionEventRequest struct {
	EventType       string   `json:"event_type"`
	OccurredAt      string   `json:"occurred_at"`
	Shares          *float64 `json:"shares"`
	Price           *float64 `json:"price"`
	Fee             float64  `json:"fee"`
	Tax             float64  `json:"tax"`
	ExpectedVersion int64    `json:"expected_version"`
	Note            string   `json:"note"`
}

func (h *PositionHandler) AddTransaction(c *gin.Context) {
	symbol := normalizePositionSymbol(c.Param("symbol"))
	var body positionEventRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	eventType := strings.ToUpper(strings.TrimSpace(body.EventType))
	if eventType != store.PositionEventBuy && eventType != store.PositionEventSell {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event_type must be BUY or SELL"})
		return
	}
	if body.Fee < 0 || body.Tax < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fee and tax must be >= 0"})
		return
	}
	occurredAt := time.Now().UTC()
	if body.OccurredAt != "" {
		parsed, err := time.Parse(time.RFC3339, body.OccurredAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "occurred_at must be RFC3339"})
			return
		}
		occurredAt = parsed
	}
	event := &store.PositionTransaction{
		Symbol: symbol, EventType: eventType, OccurredAt: occurredAt,
		Fee: body.Fee, Tax: body.Tax, Note: strings.TrimSpace(body.Note),
	}
	if body.Shares != nil {
		event.Shares = store.NewNullFloat64(*body.Shares)
	}
	if body.Price != nil {
		event.Price = store.NewNullFloat64(*body.Price)
	}
	position, err := h.repo.ApplyEvent(c.Request.Context(), event, body.ExpectedVersion)
	if errors.Is(err, store.ErrPositionVersionConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"position": position})
}

func (h *PositionHandler) Adjust(c *gin.Context) {
	symbol := normalizePositionSymbol(c.Param("symbol"))
	var body struct {
		TargetShares    float64 `json:"target_shares"`
		TargetAvgCost   float64 `json:"target_avg_cost"`
		ExpectedVersion int64   `json:"expected_version"`
		Reason          string  `json:"reason"`
		OccurredAt      string  `json:"occurred_at"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	occurredAt := time.Now().UTC()
	if body.OccurredAt != "" {
		parsed, err := time.Parse(time.RFC3339, body.OccurredAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "occurred_at must be RFC3339"})
			return
		}
		occurredAt = parsed
	}
	event := &store.PositionTransaction{
		Symbol: symbol, EventType: store.PositionEventAdjustment, OccurredAt: occurredAt,
		TargetShares:  store.NewNullFloat64(body.TargetShares),
		TargetAvgCost: store.NewNullFloat64(body.TargetAvgCost), Note: strings.TrimSpace(body.Reason),
	}
	position, err := h.repo.ApplyEvent(c.Request.Context(), event, body.ExpectedVersion)
	if errors.Is(err, store.ErrPositionVersionConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"position": position})
}

func (h *PositionHandler) ListTransactions(c *gin.Context) {
	symbol := normalizePositionSymbol(c.Param("symbol"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := h.repo.ListTransactions(c.Request.Context(), symbol, limit)
	if err != nil {
		serverError(c, h.log, err, "positions: list transactions")
		return
	}
	c.JSON(http.StatusOK, gin.H{"transactions": rows, "total": len(rows)})
}

func (h *PositionHandler) Analyze(c *gin.Context) {
	var body struct {
		Symbol       string `json:"symbol"`
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
	result, err := h.analyzer.Analyze(c.Request.Context(), body.Symbol, portfolio.AnalyzeOptions{
		Timeframe: body.Timeframe, Limit: body.Limit, ForceRefresh: body.ForceRefresh,
	})
	if err != nil {
		var upstreamErr *analysis.UpstreamStatusError
		if errors.As(err, &upstreamErr) {
			mapScoreZonesError(c, h.log, err)
			return
		}
		serverError(c, h.log, err, "position analyses: create")
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"analysis": result.Analysis, "sr_zone_analysis": result.SR, "zones": result.Zones,
	})
}

func (h *PositionHandler) ListAnalyses(c *gin.Context) {
	symbol := normalizePositionSymbol(c.Query("symbol"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	rows, err := h.repo.ListAnalyses(c.Request.Context(), symbol, limit)
	if err != nil {
		serverError(c, h.log, err, "position analyses: list")
		return
	}
	c.JSON(http.StatusOK, gin.H{"analyses": rows, "total": len(rows)})
}

func (h *PositionHandler) GetAnalysis(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	row, err := h.repo.GetAnalysis(c.Request.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "position analysis not found"})
		return
	}
	if err != nil {
		serverError(c, h.log, err, "position analyses: get")
		return
	}
	c.JSON(http.StatusOK, gin.H{"analysis": row})
}
