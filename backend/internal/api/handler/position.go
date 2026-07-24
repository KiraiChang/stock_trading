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

	"github.com/trading/backend/internal/store"
)

type PositionHandler struct {
	repo       store.PositionRepo
	portfolios store.PortfolioRepo
	log        *zap.Logger
}

func NewPositionHandler(repo store.PositionRepo, portfolios store.PortfolioRepo, log *zap.Logger) *PositionHandler {
	return &PositionHandler{repo: repo, portfolios: portfolios, log: log}
}

func normalizePositionSymbol(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}

func portfolioIDFromQuery(c *gin.Context) (uint64, bool) {
	raw := strings.TrimSpace(c.Query("portfolio_id"))
	if raw == "" {
		return store.DefaultPortfolioID, true
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "portfolio_id must be a positive integer"})
		return 0, false
	}
	return id, true
}

func portfolioIDFromRequest(c *gin.Context, bodyPortfolioID uint64) (uint64, bool) {
	if bodyPortfolioID > 0 {
		return bodyPortfolioID, true
	}
	return portfolioIDFromQuery(c)
}

func requirePortfolioAccess(c *gin.Context, portfolios store.PortfolioRepo, portfolioID uint64, write bool) bool {
	if portfolios == nil {
		return true
	}
	userID, ok := currentUserID(c)
	if !ok {
		return false
	}
	allowed, err := portfolios.CanAccess(c.Request.Context(), userID, portfolioID, write)
	if err != nil {
		serverError(c, nil, err, "portfolios: access")
		return false
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "portfolio access denied"})
		return false
	}
	return true
}

func (h *PositionHandler) List(c *gin.Context) {
	portfolioID, ok := portfolioIDFromQuery(c)
	if !ok {
		return
	}
	if !requirePortfolioAccess(c, h.portfolios, portfolioID, false) {
		return
	}
	rows, err := h.repo.List(c.Request.Context(), portfolioID)
	if err != nil {
		serverError(c, h.log, err, "positions: list")
		return
	}
	c.JSON(http.StatusOK, gin.H{"portfolio_id": portfolioID, "positions": rows, "total": len(rows)})
}

func (h *PositionHandler) Get(c *gin.Context) {
	portfolioID, ok := portfolioIDFromQuery(c)
	if !ok {
		return
	}
	if !requirePortfolioAccess(c, h.portfolios, portfolioID, false) {
		return
	}
	symbol := normalizePositionSymbol(c.Param("symbol"))
	row, err := h.repo.Get(c.Request.Context(), portfolioID, symbol)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusOK, gin.H{"position": gin.H{
			"portfolio_id": portfolioID, "symbol": symbol, "shares": 0, "avg_cost": 0, "realized_pnl": 0, "version": 0,
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
	PortfolioID     uint64   `json:"portfolio_id"`
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
	portfolioID := body.PortfolioID
	portfolioID, ok := portfolioIDFromRequest(c, portfolioID)
	if !ok {
		return
	}
	if !requirePortfolioAccess(c, h.portfolios, portfolioID, true) {
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
	position, err := h.repo.ApplyEvent(c.Request.Context(), portfolioID, event, body.ExpectedVersion)
	if errors.Is(err, store.ErrPositionVersionConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, store.ErrPositionInvalidEvent) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		serverError(c, h.log, err, "positions: add transaction")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"position": position})
}

func (h *PositionHandler) Adjust(c *gin.Context) {
	symbol := normalizePositionSymbol(c.Param("symbol"))
	var body struct {
		PortfolioID     uint64  `json:"portfolio_id"`
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
	portfolioID := body.PortfolioID
	portfolioID, ok := portfolioIDFromRequest(c, portfolioID)
	if !ok {
		return
	}
	if !requirePortfolioAccess(c, h.portfolios, portfolioID, true) {
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
	position, err := h.repo.ApplyEvent(c.Request.Context(), portfolioID, event, body.ExpectedVersion)
	if errors.Is(err, store.ErrPositionVersionConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, store.ErrPositionInvalidEvent) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		serverError(c, h.log, err, "positions: adjust")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"position": position})
}

func (h *PositionHandler) ListTransactions(c *gin.Context) {
	portfolioID, ok := portfolioIDFromQuery(c)
	if !ok {
		return
	}
	if !requirePortfolioAccess(c, h.portfolios, portfolioID, false) {
		return
	}
	symbol := normalizePositionSymbol(c.Param("symbol"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := h.repo.ListTransactions(c.Request.Context(), portfolioID, symbol, limit)
	if err != nil {
		serverError(c, h.log, err, "positions: list transactions")
		return
	}
	c.JSON(http.StatusOK, gin.H{"portfolio_id": portfolioID, "transactions": rows, "total": len(rows)})
}
