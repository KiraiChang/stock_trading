package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
)

type StockSymbolHandler struct {
	repo store.StockSymbolRepo
	log  *zap.Logger
}

func NewStockSymbolHandler(repo store.StockSymbolRepo, log *zap.Logger) *StockSymbolHandler {
	return &StockSymbolHandler{repo: repo, log: log}
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
