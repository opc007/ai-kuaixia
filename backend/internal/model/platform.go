package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Platform 平台配置
type Platform struct {
	ID          uuid.UUID `gorm:"type:char(36);primary_key" json:"id"`
	Name        string    `gorm:"uniqueIndex;size:32" json:"name"`        // douyin/kuaishou/bilibili
	DisplayName string    `gorm:"size:50" json:"display_name"`            // 抖音/快手/B站
	Domains     string    `gorm:"type:text" json:"domains"`               // 域名列表
	ParseType   string    `gorm:"size:32" json:"parse_type"`              // api/share_link/cookie
	ParseConfig string    `gorm:"type:text" json:"parse_config"`          // 解析配置
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	SortOrder   int       `gorm:"default:0" json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AIPreset AI预设配置
type AIPreset struct {
	ID          uuid.UUID `gorm:"type:char(36);primary_key" json:"id"`
	Name        string    `gorm:"uniqueIndex;size:50" json:"name"`        // intent_classification/format_result/error_handler
	DisplayName string    `gorm:"size:100" json:"display_name"`
	Content     string    `gorm:"type:text" json:"content"`
	Description string    `gorm:"size:255" json:"description"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreditPackage 积分套餐
type CreditPackage struct {
	ID        uuid.UUID `gorm:"type:char(36);primary_key" json:"id"`
	Name      string    `gorm:"size:50" json:"name"`           // 基础包/标准包/豪华包
	Price     float64   `gorm:"type:decimal(10,2)" json:"price"`
	Credits   int       `json:"credits"`                       // 基础积分
	Bonus     int       `gorm:"default:0" json:"bonus"`        // 赠送积分
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	SortOrder int       `gorm:"default:0" json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (p *Platform) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

func (ap *AIPreset) BeforeCreate(tx *gorm.DB) error {
	if ap.ID == uuid.Nil {
		ap.ID = uuid.New()
	}
	return nil
}

func (cp *CreditPackage) BeforeCreate(tx *gorm.DB) error {
	if cp.ID == uuid.Nil {
		cp.ID = uuid.New()
	}
	return nil
}
