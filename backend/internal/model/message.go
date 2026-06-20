package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ChatSession 对话会话
type ChatSession struct {
	ID        uuid.UUID `gorm:"type:char(36);primary_key" json:"id"`
	UserID    uuid.UUID `gorm:"index;type:char(36)" json:"user_id"`
	Title     string    `gorm:"size:255" json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ChatMessage 对话消息
type ChatMessage struct {
	ID        uuid.UUID `gorm:"type:char(36);primary_key" json:"id"`
	SessionID uuid.UUID `gorm:"index;type:char(36)" json:"session_id"`
	UserID    uuid.UUID `gorm:"index;type:char(36)" json:"user_id"`
	Role      string    `gorm:"size:16;not null" json:"role"` // user/assistant/system
	Content   string    `gorm:"type:text" json:"content"`
	Metadata  string    `gorm:"type:text" json:"metadata"` // 额外数据（解析结果、任务ID等）
	CreatedAt time.Time `json:"created_at"`
}

// AIServiceLog AI服务调用日志
type AIServiceLog struct {
	ID           uuid.UUID `gorm:"type:char(36);primary_key" json:"id"`
	UserID       uuid.UUID `gorm:"index;type:char(36)" json:"user_id"`
	SessionID    uuid.UUID `gorm:"index;type:char(36)" json:"session_id"`
	PresetName   string    `gorm:"size:50" json:"preset_name"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	CreditsCost  float64   `gorm:"type:decimal(10,4)" json:"credits_cost"`
	ResponseTime int       `json:"response_time"` // 毫秒
	Status       string    `gorm:"size:16" json:"status"` // success/failed
	ErrorMessage string    `gorm:"type:text" json:"error_message"`
	CreatedAt    time.Time `json:"created_at"`
}

func (cs *ChatSession) BeforeCreate(tx *gorm.DB) error {
	if cs.ID == uuid.Nil {
		cs.ID = uuid.New()
	}
	return nil
}

func (cm *ChatMessage) BeforeCreate(tx *gorm.DB) error {
	if cm.ID == uuid.Nil {
		cm.ID = uuid.New()
	}
	return nil
}

func (asl *AIServiceLog) BeforeCreate(tx *gorm.DB) error {
	if asl.ID == uuid.Nil {
		asl.ID = uuid.New()
	}
	return nil
}
