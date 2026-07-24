package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
)

type GroupHandler struct {
	repo store.GroupRepo
	log  *zap.Logger
}

func NewGroupHandler(repo store.GroupRepo, log *zap.Logger) *GroupHandler {
	return &GroupHandler{repo: repo, log: log}
}

func (h *GroupHandler) List(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	rows, err := h.repo.ListForUser(c.Request.Context(), userID)
	if err != nil {
		serverError(c, h.log, err, "groups: list")
		return
	}
	c.JSON(http.StatusOK, gin.H{"groups": rows, "total": len(rows)})
}

func (h *GroupHandler) Create(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
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
	row, err := h.repo.Create(c.Request.Context(), userID, name)
	if errors.Is(err, store.ErrGroupAccessDenied) {
		c.JSON(http.StatusForbidden, gin.H{"error": "group access denied"})
		return
	}
	if err != nil {
		serverError(c, h.log, err, "groups: create")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"group": row})
}

func (h *GroupHandler) AddMember(c *gin.Context) {
	actorID, ok := currentUserID(c)
	if !ok {
		return
	}
	groupID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || groupID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "group id must be a positive integer"})
		return
	}
	var body struct {
		UserID uint64 `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if body.UserID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}
	if err := h.repo.AddMember(c.Request.Context(), actorID, groupID, body.UserID, body.Role); err != nil {
		if errors.Is(err, store.ErrGroupAccessDenied) {
			c.JSON(http.StatusForbidden, gin.H{"error": "group access denied"})
			return
		}
		serverError(c, h.log, err, "groups: add member")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
