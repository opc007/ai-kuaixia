package handler

import (
	"net/http"

	"github.com/aikuaixia/aikuaixia/internal/service/parser"
	"github.com/gin-gonic/gin"
)

type ParseHandler struct {
	parserSvc *parser.ParserFactory
}

func NewParseHandler(parserSvc *parser.ParserFactory) *ParseHandler {
	return &ParseHandler{parserSvc: parserSvc}
}

// Parse 解析视频
func (h *ParseHandler) Parse(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.parserSvc.Parse(req.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !result.Success {
		c.JSON(http.StatusBadRequest, gin.H{"error": result.Error})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": result.Data})
}

// GetPlatforms 获取支持的平台
func (h *ParseHandler) GetPlatforms(c *gin.Context) {
	platforms := h.parserSvc.GetSupportedPlatforms()
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": platforms})
}
