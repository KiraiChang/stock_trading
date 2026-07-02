package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
)

type UserHandler struct {
	userRepo store.UserRepo
	log      *zap.Logger
}

func NewUserHandler(userRepo store.UserRepo, log *zap.Logger) *UserHandler {
	return &UserHandler{userRepo: userRepo, log: log}
}

type userResponse struct {
	ID        uint64 `json:"id"`
	Email     string `json:"email"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// GET /api/v1/users
func (h *UserHandler) List(c *gin.Context) {
	users, err := h.userRepo.List(c.Request.Context())
	if err != nil {
		serverError(c, h.log, err, "user: list")
		return
	}
	resp := make([]userResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, userResponse{
			ID:        u.ID,
			Email:     u.Email,
			Status:    u.Status,
			CreatedAt: u.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	c.JSON(http.StatusOK, gin.H{"users": resp})
}

// PATCH /api/v1/users/:id/status
func (h *UserHandler) UpdateStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var body struct {
		Status string `json:"status" binding:"required,oneof=active inactive"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be 'active' or 'inactive'"})
		return
	}

	if err := h.userRepo.UpdateStatus(c.Request.Context(), id, body.Status); err != nil {
		serverError(c, h.log, err, "user: update status")
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": id, "status": body.Status})
}
