package seed

import (
	"log"

	"github.com/aikuaixia/aikuaixia/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Run 在启动时为关键字典表种入基础数据。
// 行为：表为空时插入；已有数据则不动，避免覆盖运营修改。
func Run(db *gorm.DB) error {
	if err := seedPlatforms(db); err != nil {
		return err
	}
	if err := seedCreditPackages(db); err != nil {
		return err
	}
	if err := seedAIPresets(db); err != nil {
		return err
	}
	return nil
}

func seedPlatforms(db *gorm.DB) error {
	var n int64
	if err := db.Model(&model.Platform{}).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	platforms := []model.Platform{
		{Name: "douyin", DisplayName: "抖音", Domains: "douyin.com,iesdouyin.com,v.douyin.com", ParseType: "share_link", SortOrder: 1, IsActive: true},
		{Name: "kuaishou", DisplayName: "快手", Domains: "kuaishou.com,gifshow.com,v.kuaishou.com", ParseType: "share_link", SortOrder: 2, IsActive: true},
		{Name: "bilibili", DisplayName: "B站", Domains: "bilibili.com,b23.tv", ParseType: "share_link", SortOrder: 3, IsActive: true},
		{Name: "xiaohongshu", DisplayName: "小红书", Domains: "xiaohongshu.com,xhslink.com", ParseType: "share_link", SortOrder: 4, IsActive: true},
		{Name: "youtube", DisplayName: "YouTube", Domains: "youtube.com,youtu.be", ParseType: "share_link", SortOrder: 5, IsActive: true},
		{Name: "tiktok", DisplayName: "TikTok", Domains: "tiktok.com,vm.tiktok.com", ParseType: "share_link", SortOrder: 6, IsActive: true},
		{Name: "weishi", DisplayName: "微视", Domains: "weishi.qq.com", ParseType: "share_link", SortOrder: 7, IsActive: true},
		{Name: "xigua", DisplayName: "西瓜视频", Domains: "ixigua.com", ParseType: "share_link", SortOrder: 8, IsActive: true},
		{Name: "generic", DisplayName: "网页视频", Domains: "*", ParseType: "generic", SortOrder: 99, IsActive: true},
	}
	for i := range platforms {
		platforms[i].ID = uuid.New()
	}
	if err := db.Create(&platforms).Error; err != nil {
		return err
	}
	log.Printf("seed: 插入 %d 个平台", len(platforms))
	return nil
}

func seedCreditPackages(db *gorm.DB) error {
	var n int64
	if err := db.Model(&model.CreditPackage{}).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	pkgs := []model.CreditPackage{
		{Name: "体验包", Price: 1.0, Credits: 10, Bonus: 0, SortOrder: 1, IsActive: true},
		{Name: "基础包", Price: 9.9, Credits: 100, Bonus: 10, SortOrder: 2, IsActive: true},
		{Name: "标准包", Price: 29.9, Credits: 300, Bonus: 50, SortOrder: 3, IsActive: true},
		{Name: "豪华包", Price: 99.9, Credits: 1000, Bonus: 200, SortOrder: 4, IsActive: true},
		{Name: "超级包", Price: 299.0, Credits: 3000, Bonus: 1000, SortOrder: 5, IsActive: true},
	}
	for i := range pkgs {
		pkgs[i].ID = uuid.New()
	}
	if err := db.Create(&pkgs).Error; err != nil {
		return err
	}
	log.Printf("seed: 插入 %d 个积分套餐", len(pkgs))
	return nil
}

func seedAIPresets(db *gorm.DB) error {
	var n int64
	if err := db.Model(&model.AIPreset{}).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	presets := []model.AIPreset{
		{
			Name:        "intent_classification",
			DisplayName: "意图识别",
			Description: "用于识别用户消息意图（解析链接 / 批量下载 / 查询任务 / 帮助 / 通用对话）",
			Content: `你是一个视频下载助手"AI快侠"的意图识别模块。
根据用户输入，判断其意图类别：
- parse_link: 用户提供了任何HTTP/HTTPS链接，想要下载视频
- batch_download: 用户想要批量下载多个视频
- query_task: 用户查询下载任务状态
- help: 用户需要帮助
- general: 其他通用对话

注意：只要用户消息中包含http://或https://链接，就应该返回parse_link。
只返回意图类别名称，不要返回其他内容。`,
			IsActive: true,
		},
		{
			Name:        "format_result",
			DisplayName: "结果展示",
			Description: "用于向用户友好地展示视频解析结果",
			Content: `你是一个视频下载助手"AI快侠"。请用简洁友好的语言向用户展示视频解析结果。`,
			IsActive: true,
		},
		{
			Name:        "error_handler",
			DisplayName: "错误处理",
			Description: "当解析失败时，引导用户排查问题",
			Content: `你是一个视频下载助手"AI快侠"。当解析失败时，请用友好的语言告知用户可能的原因。`,
			IsActive: true,
		},
		{
			Name:        "general_chat",
			DisplayName: "通用对话",
			Description: "默认通用对话系统提示词",
			Content: `你是"AI快侠"，一个专业的视频下载助手。`,
			IsActive: true,
		},
	}
	for i := range presets {
		presets[i].ID = uuid.New()
	}
	if err := db.Create(&presets).Error; err != nil {
		return err
	}
	log.Printf("seed: 插入 %d 个 AI 预设", len(presets))
	return nil
}
