package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DownloadTask 下载任务
type DownloadTask struct {
	ID          uuid.UUID  `gorm:"type:char(36);primary_key" json:"id"`
	UserID      uuid.UUID  `gorm:"index;type:char(36)" json:"user_id"`
	Platform    string     `gorm:"size:32" json:"platform"`    // douyin/kuaishou/bilibili/youtube
	OriginalURL string     `gorm:"type:text" json:"original_url"`
	VideoURL    string     `gorm:"type:text" json:"video_url"`
	CoverURL    string     `gorm:"type:text" json:"cover_url"`
	Title       string     `gorm:"type:text" json:"title"`
	Author      string     `gorm:"size:128" json:"author"`
	Duration    int        `json:"duration"`                        // 视频时长（秒）
	FileSize    int64      `json:"file_size"`
	Status      string     `gorm:"size:16;default:pending" json:"status"` // pending/parsing/downloading/completed/failed
	Progress    int        `gorm:"default:0" json:"progress"`       // 0-100
	FilePath    string     `gorm:"type:text" json:"file_path"`      // 存储路径
	ErrorMsg    string     `gorm:"type:text" json:"error_msg"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// DownloadTask 下载任务
func (dt *DownloadTask) BeforeCreate(tx *gorm.DB) error {
	if dt.ID == uuid.Nil {
		dt.ID = uuid.New()
	}
	return nil
}
