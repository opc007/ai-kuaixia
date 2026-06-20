package handler

import (
	"net/http"
	"strconv"

	"github.com/aikuaixia/aikuaixia/internal/model"
	"github.com/aikuaixia/aikuaixia/internal/service/payment"
	"github.com/aikuaixia/aikuaixia/internal/service/user"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PaymentHandler struct {
	paySvc    *payment.PaymentService
	creditSvc *user.CreditService
}

func NewPaymentHandler(paySvc *payment.PaymentService, creditSvc *user.CreditService) *PaymentHandler {
	return &PaymentHandler{paySvc: paySvc, creditSvc: creditSvc}
}

// GetPackages 获取充值套餐
func (h *PaymentHandler) GetPackages(c *gin.Context) {
	packages, err := h.paySvc.GetPackages()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取套餐失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": packages})
}

// GetPaymentMethods 获取支付方式
func (h *PaymentHandler) GetPaymentMethods(c *gin.Context) {
	methods := h.paySvc.GetPaymentMethods()
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": methods})
}

// CreateRechargeOrder 创建充值订单
func (h *PaymentHandler) CreateRechargeOrder(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	if _, err := uuid.Parse(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户ID"})
		return
	}

	var req struct {
		PackageID     string `json:"package_id" binding:"required"`
		PaymentMethod string `json:"payment_method" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.paySvc.CreateRechargeOrder(userID, req.PackageID, req.PaymentMethod)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"order_no":      resp.OrderNo,
		"pay_url":       resp.PayURL,
		"pay_params":    resp.PayParams,
		"qr_code":       resp.QRCode,
	}})
}

// HandleAlipayNotify 支付宝回调
func (h *PaymentHandler) HandleAlipayNotify(c *gin.Context) {
	params := make(map[string]string)
	c.Request.ParseForm()
	for key, values := range c.Request.Form {
		if len(values) > 0 {
			params[key] = values[0]
		}
	}

	if err := h.paySvc.HandlePayNotification("alipay", params); err != nil {
		c.String(http.StatusOK, "fail")
		return
	}

	c.String(http.StatusOK, "success")
}

// HandleWechatNotify 微信支付回调
func (h *PaymentHandler) HandleWechatNotify(c *gin.Context) {
	params := make(map[string]string)
	c.Request.ParseForm()
	for key, values := range c.Request.Form {
		if len(values) > 0 {
			params[key] = values[0]
		}
	}

	if err := h.paySvc.HandlePayNotification("wechat", params); err != nil {
		c.String(http.StatusOK, "<xml><return_code>FAIL</return_code></xml>")
		return
	}

	c.String(http.StatusOK, "<xml><return_code>SUCCESS</return_code></xml>")
}

// QueryOrderStatus 查询订单状态
// 优先从数据库查本地订单，再回退到支付渠道查询
func (h *PaymentHandler) QueryOrderStatus(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	orderNo := c.Query("order_no")
	payMethod := c.Query("payment_method")
	if orderNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少订单号"})
		return
	}

	uid, _ := uuid.Parse(userID)
	var order model.RechargeOrder
	if err := h.creditSvc.DB().Where("user_id = ? AND order_no = ?", uid, orderNo).First(&order).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
			"order_no":   order.OrderNo,
			"status":     order.Status,
			"amount":     order.Amount,
			"credits":    order.Credits,
			"paid_at":    order.PaidAt,
			"created_at": order.CreatedAt,
		}})
		return
	}

	// 数据库没有，尝试支付渠道（保持原有行为）
	status, err := h.paySvc.QueryOrderStatus(payMethod, orderNo)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "订单不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": status})
}

// GetCreditTransactions 获取积分流水
func (h *PaymentHandler) GetCreditTransactions(c *gin.Context) {
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
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	txs, total, err := h.creditSvc.GetTransactions(uid, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询流水失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"list":  txs,
		"total": total,
		"page":  page,
		"size":  pageSize,
	}})
}

// GetRechargeOrders 获取充值订单
func (h *PaymentHandler) GetRechargeOrders(c *gin.Context) {
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
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	orders, total, err := h.creditSvc.GetRechargeOrders(uid, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询订单失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"list":  orders,
		"total": total,
		"page":  page,
		"size":  pageSize,
	}})
}
