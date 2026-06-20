package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/aikuaixia/aikuaixia/internal/model"
	"github.com/aikuaixia/aikuaixia/internal/pkg/config"
	"gorm.io/gorm"
)

// WechatPayProvider 微信支付提供商
type WechatPayProvider struct {
	config *config.Config
	db     *gorm.DB
}

func NewWechatPayProvider(cfg *config.Config) *WechatPayProvider {
	return &WechatPayProvider{
		config: cfg,
	}
}

// CreatePayment 创建微信支付
func (p *WechatPayProvider) CreatePayment(ctx context.Context, order *model.RechargeOrder) (*PaymentResponse, error) {
	// 构建预付单参数
	params := map[string]string{
		"appid":            p.config.WechatAppID,
		"mch_id":           p.config.WechatMchID,
		"nonce_str":        generateNonceStr(),
		"body":             "AI快侠积分充值",
		"out_trade_no":     order.OrderNo,
		"total_fee":        fmt.Sprintf("%d", int(order.Amount*100)), // 转为分
		"spbill_create_ip": "127.0.0.1",
		"notify_url":       p.config.WechatNotifyURL,
		"trade_type":       "APP",
	}

	// 生成签名
	params["sign"] = generateSign(params, p.config.WechatAPIKey)

	// 调用微信统一下单接口获取prepay_id
	prepayID, err := p.unifiedOrder(params)
	if err != nil {
		return nil, err
	}

	// 构建APP支付参数
	appParams := map[string]string{
		"appid":     p.config.WechatAppID,
		"partnerid": p.config.WechatMchID,
		"prepayid":  prepayID,
		"package":   "Sign=WXPay",
		"noncestr":  generateNonceStr(),
		"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
	}
	appParams["sign"] = generateSign(appParams, p.config.WechatAPIKey)

	return &PaymentResponse{
		OrderNo:   order.OrderNo,
		PayParams: mapToJSON(appParams),
	}, nil
}

// VerifyNotification 验证微信回调签名
func (p *WechatPayProvider) VerifyNotification(params map[string]string) bool {
	sign := params["sign"]
	delete(params, "sign")

	// 重新计算签名
	expectedSign := generateSign(params, p.config.WechatAPIKey)
	return sign == expectedSign
}

// HandleNotification 处理微信回调
func (p *WechatPayProvider) HandleNotification(ctx context.Context, params map[string]string) error {
	// 获取关键参数
	resultCode := params["result_code"]
	outTradeNo := params["out_trade_no"]
	transactionID := params["transaction_id"]

	if resultCode != "SUCCESS" {
		return nil
	}

	// 完成订单
	_ = outTradeNo
	_ = transactionID
	return nil
}

// QueryOrder 查询微信订单
func (p *WechatPayProvider) QueryOrder(ctx context.Context, orderNo string) (*OrderStatus, error) {
	// 实际实现需要调用微信查询接口
	return &OrderStatus{
		OrderNo: orderNo,
		Status:  "pending",
	}, nil
}

func (p *WechatPayProvider) unifiedOrder(params map[string]string) (string, error) {
	// 调用微信统一下单接口
	// 返回prepay_id
	return "mock_prepay_id", nil
}

func generateNonceStr() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func generateSign(params map[string]string, apiKey string) string {
	// 生成签名
	return "mock_sign"
}

func mapToJSON(params map[string]string) string {
	// 转换为JSON字符串
	return "{}"
}
