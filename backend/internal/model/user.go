package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	ID        uuid.UUID `gorm:"type:char(36);primary_key" json:"id"`
	Username  string    `gorm:"uniqueIndex;size:50" json:"username"`
	Phone     string    `gorm:"size:20" json:"phone,omitempty"`
	Email     string    `gorm:"size:100" json:"email,omitempty"`
	Password  string    `gorm:"size:255" json:"-"`
	Nickname  string    `gorm:"size:50" json:"nickname"`
	Avatar    string    `gorm:"size:255" json:"avatar"`
	DeviceID  string    `gorm:"size:64" json:"device_id,omitempty"`
	Status    int       `gorm:"default:1" json:"status"` // 1:正常 2:禁用
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserCredits 用户积分账户
type UserCredits struct {
	ID             uuid.UUID `gorm:"type:char(36);primary_key" json:"id"`
	UserID         uuid.UUID `gorm:"uniqueIndex;type:char(36)" json:"user_id"`
	Balance        int       `gorm:"default:0" json:"balance"`          // 当前积分余额
	TotalRecharged int       `gorm:"default:0" json:"total_recharged"` // 累计充值积分
	TotalConsumed  int       `gorm:"default:0" json:"total_consumed"`  // 累计消费积分
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CreditTransaction 积分流水
type CreditTransaction struct {
	ID          uuid.UUID  `gorm:"type:char(36);primary_key" json:"id"`
	UserID      uuid.UUID  `gorm:"index;type:char(36)" json:"user_id"`
	Type        string     `gorm:"size:16;not null" json:"type"`        // recharge/consume/refund/gift
	Amount      int        `gorm:"not null" json:"amount"`              // 积分变动（正数增加，负数扣除）
	Balance     int        `gorm:"not null" json:"balance"`             // 变动后余额
	Source      string     `gorm:"size:32;not null" json:"source"`      // download/chat/batch/register/recharge
	RefID       *uuid.UUID `gorm:"type:char(36)" json:"ref_id"`        // 关联ID
	Description string     `gorm:"size:255" json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
}

// RechargeOrder 充值订单
type RechargeOrder struct {
	ID            uuid.UUID  `gorm:"type:char(36);primary_key" json:"id"`
	UserID        uuid.UUID  `gorm:"index;type:char(36)" json:"user_id"`
	OrderNo       string     `gorm:"uniqueIndex;size:64" json:"order_no"`
	PackageID     string     `gorm:"size:32" json:"package_id"`
	Amount        float64    `gorm:"type:decimal(10,2)" json:"amount"`
	Credits       int        `json:"credits"`
	PaymentMethod string     `gorm:"size:16" json:"payment_method"` // alipay/wechat
	PaymentNo     string     `gorm:"size:128" json:"payment_no"`
	Status        string     `gorm:"size:16;default:pending" json:"status"` // pending/paid/cancelled/refunded
	PaidAt        *time.Time `json:"paid_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// ConsumeOrder 消费订单
type ConsumeOrder struct {
	ID        uuid.UUID  `gorm:"type:char(36);primary_key" json:"id"`
	UserID    uuid.UUID  `gorm:"index;type:char(36)" json:"user_id"`
	OrderNo   string     `gorm:"uniqueIndex;size:64" json:"order_no"`
	Type      string     `gorm:"size:16" json:"type"` // download/chat/batch
	Credits   int        `json:"credits"`
	TaskID    *uuid.UUID `gorm:"type:char(36)" json:"task_id"`
	Status    string     `gorm:"size:16;default:pending" json:"status"` // pending/completed/failed/refunded
	CreatedAt time.Time  `json:"created_at"`
}

// BeforeCreate UUID生成
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

func (uc *UserCredits) BeforeCreate(tx *gorm.DB) error {
	if uc.ID == uuid.Nil {
		uc.ID = uuid.New()
	}
	return nil
}

func (ct *CreditTransaction) BeforeCreate(tx *gorm.DB) error {
	if ct.ID == uuid.Nil {
		ct.ID = uuid.New()
	}
	return nil
}

func (ro *RechargeOrder) BeforeCreate(tx *gorm.DB) error {
	if ro.ID == uuid.Nil {
		ro.ID = uuid.New()
	}
	return nil
}

func (co *ConsumeOrder) BeforeCreate(tx *gorm.DB) error {
	if co.ID == uuid.Nil {
		co.ID = uuid.New()
	}
	return nil
}

// PasswordResetToken 密码重置令牌（一次性，30 分钟过期）
// 用户通过 email 申请重置时生成；当前无邮件服务，token 由后端日志打印
// 供开发者手动转交给用户；生产环境应替换为真实邮件发送
type PasswordResetToken struct {
	ID        uuid.UUID `gorm:"type:char(36);primary_key" json:"id"`
	UserID    uuid.UUID `gorm:"index;type:char(36);not null" json:"user_id"`
	Email     string    `gorm:"size:100;index;not null" json:"email"`
	Token     string    `gorm:"uniqueIndex;size:64;not null" json:"token"`
	ExpiresAt time.Time `gorm:"index;not null" json:"expires_at"`
	Used      bool      `gorm:"default:false" json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

func (p *PasswordResetToken) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}
