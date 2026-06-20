package handler

import (
	"net/http"

	"github.com/aikuaixia/aikuaixia/internal/service/user"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UserProfileResponse 包含用户基础信息与积分账户
type UserProfileResponse struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar"`
	Phone     string `json:"phone,omitempty"`
	Email     string `json:"email,omitempty"`
	Status    int    `json:"status"`
	CreatedAt string `json:"created_at"`
	// 积分相关
	Credits        int `json:"credits"`
	TotalRecharged int `json:"total_recharged"`
	TotalConsumed  int `json:"total_consumed"`
}

type AuthHandler struct {
	userSvc *user.UserService
}

func NewAuthHandler(userSvc *user.UserService) *AuthHandler {
	return &AuthHandler{
		userSvc: userSvc,
	}
}

// Register 用户注册
func (h *AuthHandler) Register(c *gin.Context) {
	var req user.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.userSvc.Register(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": resp})
}

// Login 用户登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req user.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.userSvc.Login(&req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": resp})
}

// GetProfile 获取用户信息（包含积分账户）
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户ID"})
		return
	}
	user, err := h.userSvc.GetUserByID(uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户信息失败"})
		return
	}

	resp := UserProfileResponse{
		ID:        user.ID.String(),
		Username:  user.Username,
		Nickname:  user.Nickname,
		Avatar:    user.Avatar,
		Phone:     user.Phone,
		Email:     user.Email,
		Status:    user.Status,
		CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
	}

	// 积分（缺失时按 0 处理，不阻塞接口）
	if credits, err := h.userSvc.GetCredits(uid); err == nil && credits != nil {
		resp.Credits = credits.Balance
		resp.TotalRecharged = credits.TotalRecharged
		resp.TotalConsumed = credits.TotalConsumed
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": resp})
}




// ForgotPassword 通过验证码重置密码
// ForgotPassword 申请密码重置（按邮箱）
// 无邮件服务：token 写入日志，前端 demo 可直接用返回的 token 跳转重置页
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入邮箱"})
		return
	}
	token, err := h.userSvc.RequestPasswordReset(req.Email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"ok":           true,
			"token":        token,        // demo：生产不应回传，由邮件下发
			"reset_url":    "/reset-password?token=" + token,
			"expires_in":   1800,
			"delivery_note": "无邮件服务，重置链接已打印到后端日志（请查看 terminal）",
		},
	})
}

// ResetPassword 用 token 完成重置
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req struct {
		Token       string `json:"token" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误：" + err.Error()})
		return
	}
	if err := h.userSvc.ConfirmPasswordReset(req.Token, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"ok": true}})
}

// ChangePassword 已登录用户修改密码
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID := c.GetString("user_id")
	uid, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误：" + err.Error()})
		return
	}
	if err := h.userSvc.ChangePassword(uid, req.OldPassword, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"ok": true}})
}

// OAuthLogin 第三方登录占位
func (h *AuthHandler) OAuthLogin(c *gin.Context) {
	provider := c.Param("provider")
	var req struct {
		Code     string `json:"code" binding:"required"`
		Nickname string `json:"nickname"`
		Avatar   string `json:"avatar"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误：" + err.Error()})
		return
	}
	resp, err := h.userSvc.LoginByOAuth(provider, req.Code, req.Nickname, req.Avatar)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"id":       resp.ID,
			"username": resp.Username,
			"nickname": resp.Nickname,
			"token":    resp.Token,
			"provider": provider,
		},
	})
}

// SendCode 发送登录验证码
func (h *AuthHandler) SendCode(c *gin.Context) {
	var req struct {
		Phone   string `json:"phone" binding:"required"`
		Purpose string `json:"purpose"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入手机号"})
		return
	}
	code, err := h.userSvc.SendCode(req.Phone, req.Purpose)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Demo: 直接返回验证码（生产应由短信服务商发送）
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"code":  code,
			"ttl":   300,
			"debug": true,
		},
	})
}

// LoginByCode 验证码登录（未注册则自动注册）
func (h *AuthHandler) LoginByCode(c *gin.Context) {
	var req struct {
		Phone    string `json:"phone" binding:"required"`
		Code     string `json:"code" binding:"required"`
		Nickname string `json:"nickname"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入手机号和验证码"})
		return
	}
	resp, err := h.userSvc.LoginOrRegisterByCode(req.Phone, req.Code, req.Nickname)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"id":       resp.ID,
			"username": resp.Username,
			"nickname": resp.Nickname,
			"token":    resp.Token,
		},
	})
}
