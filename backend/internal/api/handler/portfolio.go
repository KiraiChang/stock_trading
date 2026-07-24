package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
)

type PortfolioHandler struct {
	repo store.PortfolioRepo
	log  *zap.Logger
}

func NewPortfolioHandler(repo store.PortfolioRepo, log *zap.Logger) *PortfolioHandler {
	return &PortfolioHandler{repo: repo, log: log}
}

func currentUserID(c *gin.Context) (uint64, bool) {
	v, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return 0, false
	}
	switch id := v.(type) {
	case uint64:
		if id > 0 {
			return id, true
		}
	case uint:
		if id > 0 {
			return uint64(id), true
		}
	case int:
		if id > 0 {
			return uint64(id), true
		}
	}
	c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user context"})
	return 0, false
}

func (h *PortfolioHandler) List(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	rows, err := h.repo.ListForUser(c.Request.Context(), userID)
	if err != nil {
		serverError(c, h.log, err, "portfolios: list")
		return
	}
	c.JSON(http.StatusOK, gin.H{"portfolios": rows, "total": len(rows)})
}

func (h *PortfolioHandler) Create(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	var body struct {
		Name    string `json:"name"`
		GroupID uint64 `json:"group_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	name := strings.TrimSpace(body.Name)
	if len(name) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name must be 128 characters or fewer"})
		return
	}
	var row *store.Portfolio
	var err error
	if body.GroupID > 0 {
		row, err = h.repo.CreateForGroup(c.Request.Context(), userID, body.GroupID, name)
	} else {
		row, err = h.repo.CreateForUser(c.Request.Context(), userID, name)
	}
	if errors.Is(err, store.ErrPortfolioAccessDenied) {
		c.JSON(http.StatusForbidden, gin.H{"error": "portfolio access denied"})
		return
	}
	if err != nil {
		serverError(c, h.log, err, "portfolios: create")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"portfolio": row})
}
