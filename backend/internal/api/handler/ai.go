package handler

import (
	"net/http"

	"github.com/aikuaixia/aikuaixia/internal/service/ai"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AIHandler struct {
	agent *ai.Agent
}

func NewAIHandler(agent *ai.Agent) *AIHandler {
	return &AIHandler{agent: agent}
}

// Chat AI对话
func (h *AIHandler) Chat(c *gin.Context) {
	userIDStr := c.GetString("user_id")
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户ID"})
		return
	}

	var req struct {
		Message   string `json:"message" binding:"required"`
		SessionID string `json:"session_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sessionID := uuid.New()
	if req.SessionID != "" {
		sessionID, _ = uuid.Parse(req.SessionID)
	}

	resp, err := h.agent.Chat(c.Request.Context(), userID, req.Message, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI服务异常"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"reply":      resp.Reply,
			"action":     resp.Action,
			"session_id": sessionID.String(),
			"video_url":  resp.VideoURL,
			"cover_url":  resp.CoverURL,
			"title":      resp.Title,
		},
	})
}

// GetHistory 获取对话历史
func (h *AIHandler) GetHistory(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": []interface{}{}})
}
