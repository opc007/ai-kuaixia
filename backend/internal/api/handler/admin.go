package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/aikuaixia/aikuaixia/internal/model"
	"github.com/aikuaixia/aikuaixia/internal/service/payment"
	"github.com/aikuaixia/aikuaixia/internal/service/user"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AdminHandler 提供管理后台所需的 CRUD 与统计接口
// 管理端没有独立的用户体系，简化：使用 /auth/login 同样的用户名密码校验身份；
// 通过 Authorization: Bearer <jwt> 鉴权后，校验 user 名是否以 "admin_" 开头（或 is_admin 标记位）。
type AdminHandler struct {
	db        *gorm.DB
	creditSvc *user.CreditService
	paySvc    *payment.PaymentService
}

func NewAdminHandler(db *gorm.DB, creditSvc *user.CreditService, paySvc *payment.PaymentService) *AdminHandler {
	return &AdminHandler{db: db, creditSvc: creditSvc, paySvc: paySvc}
}

// DashboardStats 看板数据
func (h *AdminHandler) DashboardStats(c *gin.Context) {
	var userCount, orderCount, paidCount, aiCount int64
	h.db.Model(&model.User{}).Count(&userCount)
	h.db.Model(&model.RechargeOrder{}).Count(&orderCount)
	h.db.Model(&model.RechargeOrder{}).Where("status = ?", "paid").Count(&paidCount)
	h.db.Model(&model.AIServiceLog{}).Count(&aiCount)

	// 今日收入
	var todayRevenue float64
	todayStart := time.Now().Format("2006-01-02")
	h.db.Model(&model.RechargeOrder{}).
		Where("status = ? AND paid_at IS NOT NULL AND date(paid_at) = ?", "paid", todayStart).
		Select("COALESCE(SUM(amount), 0)").Scan(&todayRevenue)

	// 平台分布
	type Row struct {
		Platform string
		Total    int64
	}
	var rows []Row
	h.db.Model(&model.DownloadTask{}).Select("platform, COUNT(*) as total").Group("platform").Scan(&rows)

	// 最新订单
	var recentOrders []model.RechargeOrder
	h.db.Order("created_at DESC").Limit(5).Find(&recentOrders)

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"user_count":     userCount,
		"order_count":    orderCount,
		"paid_count":     paidCount,
		"ai_count":       aiCount,
		"today_revenue":  todayRevenue,
		"platform_stat":  rows,
		"recent_orders":  recentOrders,
	}})
}

// ListUsers 用户列表
func (h *AdminHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	keyword := c.Query("keyword")
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	q := h.db.Model(&model.User{})
	if keyword != "" {
		q = q.Where("username LIKE ? OR phone LIKE ? OR nickname LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	var total int64
	q.Count(&total)

	var users []model.User
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&users)

	// 关联积分
	userIDs := make([]uuid.UUID, 0, len(users))
	for _, u := range users {
		userIDs = append(userIDs, u.ID)
	}
	creditsMap := map[uuid.UUID]model.UserCredits{}
	if len(userIDs) > 0 {
		var cs []model.UserCredits
		h.db.Where("user_id IN ?", userIDs).Find(&cs)
		for _, c := range cs {
			creditsMap[c.UserID] = c
		}
	}

	type UserRow struct {
		model.User
		Credits int `json:"credits"`
	}
	out := make([]UserRow, 0, len(users))
	for _, u := range users {
		row := UserRow{User: u}
		if c, ok := creditsMap[u.ID]; ok {
			row.Credits = c.Balance
		}
		out = append(out, row)
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"list":  out,
		"total": total,
		"page":  page,
		"size":  pageSize,
	}})
}

// ToggleUserStatus 启用/禁用用户
func (h *AdminHandler) ToggleUserStatus(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id" binding:"required"`
		Status int    `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	uid, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 user_id"})
		return
	}
	if req.Status == 0 {
		req.Status = 1
	}
	if err := h.db.Model(&model.User{}).Where("id = ?", uid).Update("status", req.Status).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"status": req.Status}})
}

// ListOrders 订单列表（合并充值 + 消费）
func (h *AdminHandler) ListOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	orderType := c.Query("type") // recharge / consume
	status := c.Query("status")
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	if orderType == "consume" {
		var orders []model.ConsumeOrder
		var total int64
		q := h.db.Model(&model.ConsumeOrder{})
		if status != "" {
			q = q.Where("status = ?", status)
		}
		q.Count(&total)
		q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&orders)
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"list": orders, "total": total, "page": page, "size": pageSize}})
		return
	}

	// 默认返回充值订单
	var orders []model.RechargeOrder
	var total int64
	q := h.db.Model(&model.RechargeOrder{})
	if orderType == "recharge" {
		// 已过滤，无需加 type 条件
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	q.Count(&total)
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&orders)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"list": orders, "total": total, "page": page, "size": pageSize}})
}

// ListPlatforms 平台列表
func (h *AdminHandler) ListPlatforms(c *gin.Context) {
	var platforms []model.Platform
	if err := h.db.Order("sort_order ASC").Find(&platforms).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": platforms})
}

// UpdatePlatform 启用/禁用平台
func (h *AdminHandler) UpdatePlatform(c *gin.Context) {
	var req struct {
		ID       string `json:"id" binding:"required"`
		IsActive *bool  `json:"is_active"`
		Name     string `json:"name"`
		Domains  string `json:"domains"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	uid, err := uuid.Parse(req.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	updates := map[string]interface{}{}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Domains != "" {
		updates["domains"] = req.Domains
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无更新字段"})
		return
	}
	if err := h.db.Model(&model.Platform{}).Where("id = ?", uid).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200})
}

// ListAIPresets AI 预设列表
func (h *AdminHandler) ListAIPresets(c *gin.Context) {
	var presets []model.AIPreset
	if err := h.db.Find(&presets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": presets})
}

// UpdateAIPreset 更新 AI 预设
func (h *AdminHandler) UpdateAIPreset(c *gin.Context) {
	var req struct {
		ID          string `json:"id" binding:"required"`
		DisplayName string `json:"display_name"`
		Content     string `json:"content"`
		IsActive    *bool  `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	uid, err := uuid.Parse(req.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	updates := map[string]interface{}{}
	if req.DisplayName != "" {
		updates["display_name"] = req.DisplayName
	}
	if req.Content != "" {
		updates["content"] = req.Content
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无更新字段"})
		return
	}
	if err := h.db.Model(&model.AIPreset{}).Where("id = ?", uid).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200})
}

// ListCreditPackages 套餐列表（管理端使用，复用 GET /packages）
// 这里再加一个 update 接口以便管理端修改
func (h *AdminHandler) UpdateCreditPackage(c *gin.Context) {
	var req struct {
		ID       string  `json:"id" binding:"required"`
		Name     string  `json:"name"`
		Price    float64 `json:"price"`
		Credits  int     `json:"credits"`
		Bonus    int     `json:"bonus"`
		IsActive *bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	uid, err := uuid.Parse(req.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Price > 0 {
		updates["price"] = req.Price
	}
	if req.Credits > 0 {
		updates["credits"] = req.Credits
	}
	if req.Bonus >= 0 {
		updates["bonus"] = req.Bonus
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无更新字段"})
		return
	}
	if err := h.db.Model(&model.CreditPackage{}).Where("id = ?", uid).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200})
}
