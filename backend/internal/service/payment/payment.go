package payment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aikuaixia/aikuaixia/internal/model"
	"github.com/aikuaixia/aikuaixia/internal/pkg/config"
	"github.com/aikuaixia/aikuaixia/internal/service/user"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PaymentProvider 支付提供商接口
type PaymentProvider interface {
	// CreatePayment 创建支付单
	CreatePayment(ctx context.Context, order *model.RechargeOrder) (*PaymentResponse, error)
	// VerifyNotification 验证回调签名
	VerifyNotification(params map[string]string) bool
	// HandleNotification 处理支付回调
	HandleNotification(ctx context.Context, params map[string]string) error
	// QueryOrder 查询订单状态
	QueryOrder(ctx context.Context, orderNo string) (*OrderStatus, error)
}

// PaymentResponse 支付响应
type PaymentResponse struct {
	OrderNo     string `json:"order_no"`
	PayURL      string `json:"pay_url,omitempty"`      // 支付链接
	PayParams   string `json:"pay_params,omitempty"`    // 支付参数（APP支付）
	QRCode      string `json:"qr_code,omitempty"`       // 二维码
}

// OrderStatus 订单状态
type OrderStatus struct {
	OrderNo   string `json:"order_no"`
	Status    string `json:"status"`
	PaymentNo string `json:"payment_no"`
}

// PaymentService 支付服务
type PaymentService struct {
	db         *gorm.DB
	config     *config.Config
	creditSvc  *user.CreditService
	providers  map[string]PaymentProvider
}

func NewPaymentService(db *gorm.DB, cfg *config.Config, creditSvc *user.CreditService) *PaymentService {
	svc := &PaymentService{
		db:        db,
		config:    cfg,
		creditSvc: creditSvc,
		providers: make(map[string]PaymentProvider),
	}

	// 注册支付提供商
	svc.providers["alipay"] = NewAlipayProvider(cfg)
	svc.providers["wechat"] = NewWechatPayProvider(cfg)

	return svc
}

// CreateRechargeOrder 创建充值订单
func (s *PaymentService) CreateRechargeOrder(userID string, packageID, payMethod string) (*PaymentResponse, error) {
	// 验证支付方式
	if _, ok := s.providers[payMethod]; !ok {
		return nil, errors.New("不支持的支付方式")
	}

	// 解析用户ID
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, errors.New("无效的用户ID")
	}

	// 创建订单
	order, err := s.creditSvc.CreateRechargeOrder(uid, packageID, payMethod)
	if err != nil {
		return nil, err
	}

	// 调用支付渠道
	provider := s.providers[payMethod]
	return provider.CreatePayment(context.Background(), order)
}

// HandlePayNotification 处理支付回调
func (s *PaymentService) HandlePayNotification(payMethod string, params map[string]string) error {
	provider, ok := s.providers[payMethod]
	if !ok {
		return errors.New("不支持的支付方式")
	}

	// 验签
	if !provider.VerifyNotification(params) {
		return errors.New("签名验证失败")
	}

	// 处理回调
	return provider.HandleNotification(context.Background(), params)
}

// QueryOrderStatus 查询订单状态
func (s *PaymentService) QueryOrderStatus(payMethod, orderNo string) (*OrderStatus, error) {
	provider, ok := s.providers[payMethod]
	if !ok {
		return nil, errors.New("不支持的支付方式")
	}

	return provider.QueryOrder(context.Background(), orderNo)
}

// GetPackages 获取充值套餐
func (s *PaymentService) GetPackages() ([]model.CreditPackage, error) {
	var packages []model.CreditPackage
	if err := s.db.Where("is_active = true").Order("sort_order ASC").Find(&packages).Error; err != nil {
		return nil, err
	}
	return packages, nil
}

// GetPaymentMethods 获取支持的支付方式
func (s *PaymentService) GetPaymentMethods() []map[string]string {
	return []map[string]string{
		{"id": "alipay", "name": "支付宝", "icon": "alipay"},
		{"id": "wechat", "name": "微信支付", "icon": "wechat"},
	}
}

// CreateConsumeOrder 创建消费订单
func (s *PaymentService) CreateConsumeOrder(userID uuid.UUID, consumeType string, credits int, taskID *uuid.UUID) (*model.ConsumeOrder, error) {
	order := &model.ConsumeOrder{
		UserID:  userID,
		OrderNo: fmt.Sprintf("CS%s", time.Now().Format("20060102150405")),
		Type:    consumeType,
		Credits: credits,
		TaskID:  taskID,
		Status:  "pending",
	}

	if err := s.db.Create(order).Error; err != nil {
		return nil, err
	}

	return order, nil
}

// CompleteConsumeOrder 完成消费订单
func (s *PaymentService) CompleteConsumeOrder(orderNo string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var order model.ConsumeOrder
		if err := tx.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			return errors.New("订单不存在")
		}

		if order.Status != "pending" {
			return nil
		}

		order.Status = "completed"
		return tx.Save(&order).Error
	})
}

// RefundConsumeOrder 退还积分
func (s *PaymentService) RefundConsumeOrder(orderNo string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var order model.ConsumeOrder
		if err := tx.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			return errors.New("订单不存在")
		}

		if order.Status != "completed" {
			return errors.New("订单状态异常")
		}

		order.Status = "refunded"
		if err := tx.Save(&order).Error; err != nil {
			return err
		}

		// 退还积分
		return s.creditSvc.AddCredits(order.UserID, order.Credits, "refund", &order.ID, "退款退还积分")
	})
}
