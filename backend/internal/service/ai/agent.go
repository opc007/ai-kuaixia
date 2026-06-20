package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/aikuaixia/aikuaixia/internal/model"
	"github.com/aikuaixia/aikuaixia/internal/pkg/config"
	"github.com/aikuaixia/aikuaixia/internal/service/parser"
	"github.com/aikuaixia/aikuaixia/internal/service/user"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Agent AI智能体
type Agent struct {
	db          *gorm.DB
	config      *config.Config
	parserSvc   *parser.ParserFactory
	creditSvc   *user.CreditService
	minimax     *MiniMaxClient
	presets     map[string]string
}

func NewAgent(db *gorm.DB, cfg *config.Config, parserSvc *parser.ParserFactory, creditSvc *user.CreditService) *Agent {
	agent := &Agent{
		db:        db,
		config:    cfg,
		parserSvc: parserSvc,
		creditSvc: creditSvc,
		minimax:   NewMiniMaxClient(cfg.MiniMaxAPIKey, cfg.MiniMaxModel),
		presets:   make(map[string]string),
	}

	// 加载预设
	agent.loadPresets()

	return agent
}

// loadPresets 加载预设
func (a *Agent) loadPresets() {
	a.presets["intent_classification"] = `你是一个视频下载助手"AI快侠"的意图识别模块。
根据用户输入，判断其意图类别：
- parse_link: 用户提供了任何HTTP/HTTPS链接，想要下载视频
- batch_download: 用户想要批量下载多个视频
- query_task: 用户查询下载任务状态
- help: 用户需要帮助
- general: 其他通用对话

注意：只要用户消息中包含http://或https://链接，就应该返回parse_link。
只返回意图类别名称，不要返回其他内容。`

	a.presets["format_result"] = `你是一个视频下载助手"AI快侠"。请用简洁友好的语言向用户展示视频解析结果。
包含：标题、作者、时长、画质选项。
提醒用户可以点击下载按钮保存到相册。
如果用户积分不足，提醒用户充值。`

	a.presets["error_handler"] = `你是一个视频下载助手"AI快侠"。当解析失败时，请用友好的语言告知用户可能的原因：
- 链接无效或已过期
- 页面使用JavaScript动态加载（当前版本暂不支持）
- 视频已被删除或设为私密
- 网络问题

并给出建议的解决方案：
1. 检查链接是否正确
2. 尝试使用其他平台的视频链接
3. 对于JavaScript动态加载的网站，建议使用录屏工具
4. 检查网络连接`

	a.presets["general_chat"] = `你是"AI快侠"，一个专业的视频下载助手。你可以帮助用户：
1. 下载任意网页视频（支持任何HTTP/HTTPS链接）
2. 智能识别视频内容并提取下载链接
3. 支持主流平台：抖音、快手、B站、小红书、YouTube、TikTok等
4. 支持其他网站的嵌入视频

请用简洁友好的语言回答用户问题。
如果用户没有提供链接，引导他们复制视频分享链接。
重要：不要限制用户使用特定平台，任何包含视频的网页链接都可以尝试解析。

计费规则：
- 视频下载：1积分/次
- AI对话：0.5积分/次

请用中文回复。`

	// 从数据库加载预设（如果有）
	var presets []model.AIPreset
	if err := a.db.Where("is_active = true").Find(&presets); err == nil {
		for _, preset := range presets {
			a.presets[preset.Name] = preset.Content
		}
	}
}

// Chat AI对话
func (a *Agent) Chat(ctx context.Context, userID uuid.UUID, message string, sessionID uuid.UUID) (*ChatResponse, error) {
	// 1. 检查积分
	balance, err := a.creditSvc.GetBalance(userID)
	if err != nil {
		return nil, errors.New("获取积分失败")
	}

	if balance < 1 {
		return &ChatResponse{
			Reply:  "您的积分不足，请先充值后再使用。",
			Action: "insufficient_credits",
		}, nil
	}

	// 2. 识别意图
	intent, err := a.classifyIntent(message)
	if err != nil {
		intent = "general"
	}

	// 3. 根据意图处理
	var response *ChatResponse

	switch intent {
	case "parse_link":
		response, err = a.handleParseLink(userID, message, sessionID)
	case "batch_download":
		response, err = a.handleBatchDownload(userID, message, sessionID)
	case "query_task":
		response, err = a.handleQueryTask(userID, message, sessionID)
	case "help":
		response, err = a.handleHelp(message, sessionID)
	default:
		response, err = a.handleGeneral(userID, message, sessionID)
	}

	if err != nil {
		return nil, err
	}

	// 4. 扣除积分（AI对话）
	if response.Action != "insufficient_credits" {
		a.creditSvc.DeductCredits(userID, 1, "chat", nil)
	}

	// 5. 保存消息
	a.saveMessage(sessionID, userID, "user", message)
	a.saveMessage(sessionID, userID, "assistant", response.Reply)

	return response, nil
}

// classifyIntent 意图识别
// 当 AI 服务不可用时（如未配置 API Key），降级为规则匹配，保证用户仍可使用核心功能。
func (a *Agent) classifyIntent(message string) (string, error) {
	msg := strings.TrimSpace(strings.ToLower(message))
	// 规则优先：链接类
	if strings.Contains(message, "http://") || strings.Contains(message, "https://") ||
		strings.Contains(msg, "v.douyin.com") || strings.Contains(msg, "v.kuaishou.com") ||
		strings.Contains(msg, "b23.tv") || strings.Contains(msg, "xhslink.com") ||
		strings.Contains(msg, "youtu.be") || strings.Contains(msg, "vm.tiktok.com") {
		return "parse_link", nil
	}
	if strings.Contains(msg, "批量") || strings.Contains(msg, "多个") || strings.Contains(msg, "batch") {
		return "batch_download", nil
	}
	if strings.Contains(msg, "查询") || strings.Contains(msg, "进度") || strings.Contains(msg, "任务") || strings.Contains(msg, "下载记录") {
		return "query_task", nil
	}
	if strings.Contains(msg, "帮助") || strings.Contains(msg, "怎么用") || strings.Contains(msg, "help") || strings.Contains(msg, "使用") {
		return "help", nil
	}

	// 没有 AI Key，直接规则
	if a.minimax == nil || a.minimax.apiKey == "" {
		return "general", nil
	}

	prompt := a.presets["intent_classification"]
	resp, err := a.minimax.Chat([]Message{
		{Role: "system", Content: prompt},
		{Role: "user", Content: message},
	})
	if err != nil {
		// AI 不可用时降级为 general，不要把异常抛给上层
		return "general", nil
	}

	intent := strings.TrimSpace(resp)
	validIntents := []string{"parse_link", "batch_download", "query_task", "help", "general"}
	for _, valid := range validIntents {
		if intent == valid {
			return intent, nil
		}
	}

	return "general", nil
}

// handleParseLink 处理视频链接解析 - ReAct 多步兜底
// 步骤1: 内置平台解析器 (douyin/bilibili/...)
// 步骤2: 第三方解析 (cobalt)
// 步骤3: AI 直接看页面 HTML 找视频链接
// 步骤4: 友好提示 + 解决方案
func (a *Agent) handleParseLink(userID uuid.UUID, message string, sessionID uuid.UUID) (*ChatResponse, error) {
	// 提取URL
	url := extractURL(message)
	if url == "" {
		return &ChatResponse{
			Reply:  "您好！请提供视频链接，我来帮您下载。\n\n支持任意网页视频链接，包括抖音、快手、B站、小红书、YouTube、TikTok等平台。",
			Action: "need_url",
		}, nil
	}

	// ===== 步骤 1: 内置平台解析器 =====
	result, err := a.parserSvc.Parse(url)
	if err == nil && result != nil && result.Success && result.Data != nil && result.Data.VideoURL != "" {
		return a.buildParseSuccessReply(result.Data, "1"), nil
	}

	// ===== 步骤 2: 第三方解析 (cobalt 等) =====
	// parserSvc 已经内部 fallback 到 thirdparty，这里补充一下 yt-dlp 兜底
	if ytURL, ytTitle, ytAuthor, ok := a.tryYtDlpDirect(url); ok {
		return &ChatResponse{
			Reply: fmt.Sprintf("✅ 视频解析成功（通用模式）！\n\n📝 标题：%s\n👤 作者：%s\n\n请点击下方下载按钮。",
				ytTitle, ytAuthor),
			Action:   "parse_complete",
			VideoURL: ytURL,
			Title:    ytTitle,
		}, nil
	}

	// ===== 步骤 3: AI 直接读页面找视频链接 =====
	if extractedURL, title, ok := a.aiExtractVideoFromPage(url); ok {
		return &ChatResponse{
			Reply: fmt.Sprintf("✅ AI 已从页面识别到视频链接！\n\n📝 标题：%s\n\n请点击下方下载按钮。", title),
			Action:   "parse_complete",
			VideoURL: extractedURL,
			Title:    title,
		}, nil
	}

	// ===== 步骤 4: 友好兜底 =====
	return &ChatResponse{
		Reply: fmt.Sprintf("😅 抱歉，我暂时无法解析这个链接。\n\n可能的原因：\n• 链接需要登录才能访问\n• 视频是私密的\n• 网站使用了我还不支持的技术\n\n建议：\n1. 确认链接是公开的\n2. 试试其他平台的视频\n3. 加群反馈：ai快侠用户群"),
		Action: "parse_failed",
	}, nil
}

// buildParseSuccessReply 构建解析成功的回复
func (a *Agent) buildParseSuccessReply(d *parser.VideoInfo, via string) *ChatResponse {
	viaText := ""
	switch via {
	case "1":
		viaText = ""
	}
	reply := fmt.Sprintf("✅ 视频解析成功%s！\n\n📱 平台：%s\n📝 标题：%s\n👤 作者：%s\n⏱ 时长：%d秒\n\n请点击下方下载按钮保存。",
		viaText, d.Platform, d.Title, d.Author, d.Duration)
	return &ChatResponse{
		Reply:    reply,
		Action:   "parse_complete",
		VideoURL: d.VideoURL,
		CoverURL: d.CoverURL,
		Title:    d.Title,
	}
}

// tryYtDlpDirect 调 yt-dlp 拿直链
func (a *Agent) tryYtDlpDirect(url string) (videoURL, title, author string, ok bool) {
	cmd := exec.Command("yt-dlp",
		"--no-check-certificates",
		"--no-warnings",
		"--no-playlist",
		"--get-url",
		"-f", "best[ext=mp4]/best",
		url,
	)
	out, err := cmd.Output()
	if err != nil {
		return "", "", "", false
	}
	videoURL = strings.TrimSpace(string(out))
	if videoURL == "" {
		return "", "", "", false
	}
	// 再拿标题
	cmd2 := exec.Command("yt-dlp",
		"--no-check-certificates",
		"--no-warnings",
		"--no-playlist",
		"--get-title",
		"--get-uploader",
		url,
	)
	out2, err2 := cmd2.Output()
	if err2 == nil {
		lines := strings.Split(strings.TrimSpace(string(out2)), "\n")
		if len(lines) >= 1 { title = lines[0] }
		if len(lines) >= 2 { author = lines[1] }
	}
	if title == "" { title = "网页视频" }
	return videoURL, title, author, true
}

// aiExtractVideoFromPage AI 从页面 HTML 中找视频链接
func (a *Agent) aiExtractVideoFromPage(url string) (videoURL, title string, ok bool) {
	if a.minimax == nil || a.minimax.apiKey == "" {
		return "", "", false
	}
	// 抓页面
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil { return "", "", false }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { return "", "", false }
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 200*1024)) // 最多 200KB
	html := string(body)

	// 先做规则提取（避免每次都调 AI）
	rules := []struct{ pattern, prefix string;}{
		{`og:video["']\s*content=["']([^"\']+\.(mp4|m3u8)[^"\']*)["']`, ""},
		{`<source[^>]+src=["']([^"\']+\.(mp4|m3u8)[^"\']*)["']`, ""},
		{`<video[^>]+src=["']([^"\']+)["']`, ""},
		{`videoUrl["']:\s*["']([^"\']+)["']`, ""},
		{`video_url["']:\s*["']([^"\']+)["']`, ""},
	}
	for _, r := range rules {
		re := regexp.MustCompile(r.pattern)
		if m := re.FindStringSubmatch(html); len(m) > 1 {
			videoURL = strings.ReplaceAll(m[1], `\/`, `/`)
			if !strings.HasPrefix(videoURL, "http") {
				videoURL = "https:" + videoURL
			}
			// 找标题
			if tm := regexp.MustCompile(`<title[^>]*>([^<]+)</title>`).FindStringSubmatch(html); len(tm) > 1 {
				title = strings.TrimSpace(tm[1])
			}
			if title == "" { title = "网页视频" }
			return videoURL, title, true
		}
	}

	// 调 AI
	prompt := `你是一个网页视频链接提取助手。请从以下 HTML 中找到主要的视频直链 (.mp4 / .m3u8 / .webm)。
如果有多个，请选择清晰度最高、最稳定的。
只返回 JSON 格式：{"url":"...", "title":"..."}，不要其他内容。`
	resp2, err := a.minimax.Chat([]Message{
		{Role: "system", Content: prompt},
		{Role: "user", Content: html},
	})
	if err != nil { return "", "", false }
	// 简单解析（不引 json 包用 regex）
	re := regexp.MustCompile(`"url"\s*:\s*"([^"]+)"`)
	if m := re.FindStringSubmatch(resp2); len(m) > 1 {
		videoURL = strings.ReplaceAll(m[1], `\/`, `/`)
	}
	if videoURL == "" { return "", "", false }
	re2 := regexp.MustCompile(`"title"\s*:\s*"([^"]+)"`)
	if m := re2.FindStringSubmatch(resp2); len(m) > 1 { title = m[1] }
	if title == "" { title = "网页视频" }
	return videoURL, title, true
}

// handleBatchDownload 处理批量下载
func (a *Agent) handleBatchDownload(userID uuid.UUID, message string, sessionID uuid.UUID) (*ChatResponse, error) {
	return &ChatResponse{
		Reply:  "批量下载功能正在开发中，请稍后再试。",
		Action: "feature_coming_soon",
	}, nil
}

// handleQueryTask 处理任务查询
func (a *Agent) handleQueryTask(userID uuid.UUID, message string, sessionID uuid.UUID) (*ChatResponse, error) {
	// 查询用户最近的任务
	var tasks []model.DownloadTask
	a.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(5).Find(&tasks)

	if len(tasks) == 0 {
		return &ChatResponse{
			Reply:  "您还没有下载记录，快去下载第一个视频吧！",
			Action: "no_tasks",
		}, nil
	}

	reply := "📋 您最近的下载记录：\n\n"
	for i, task := range tasks {
		status := "⏳ 进行中"
		if task.Status == "completed" {
			status = "✅ 已完成"
		} else if task.Status == "failed" {
			status = "❌ 失败"
		}
		reply += fmt.Sprintf("%d. %s - %s\n", i+1, task.Title, status)
	}

	return &ChatResponse{
		Reply:  reply,
		Action: "query_result",
	}, nil
}

// handleHelp 处理帮助请求
func (a *Agent) handleHelp(message string, sessionID uuid.UUID) (*ChatResponse, error) {
	reply := `👋 您好！我是"AI快侠"视频下载助手。

📖 使用指南：
1️⃣ 复制任意网页视频链接
2️⃣ 发送给我或粘贴到输入框
3️⃣ 等待AI智能解析完成
4️⃣ 点击下载按钮保存

💰 计费规则：
• 视频下载：1积分/次
• AI对话：0.5积分/次
• 注册赠送10积分

🎯 支持范围：
• 主流平台：抖音、快手、B站、小红书、YouTube、TikTok
• 任意网页：支持任何包含视频的HTTP/HTTPS链接
• 智能识别：AI自动分析页面提取视频

💡 小技巧：
• 直接发送链接可以快速解析
• 输入"查询"查看下载记录
• 输入"充值"查看充值套餐`

	return &ChatResponse{
		Reply:  reply,
		Action: "help",
	}, nil
}

// handleGeneral 处理通用对话
// AI 不可用时给出友好的引导回复，保证服务可用。
func (a *Agent) handleGeneral(userID uuid.UUID, message string, sessionID uuid.UUID) (*ChatResponse, error) {
	// 没有配置 AI Key 时给出规则化回复
	if a.minimax == nil || a.minimax.apiKey == "" {
		return &ChatResponse{
			Reply: "您好！我是 AI快侠视频下载助手。\n\n请直接发送视频链接（支持抖音/快手/B站/小红书/YouTube 等），我帮您解析下载。\n\n输入 帮助 查看使用指南。",
			Action: "chat_offline",
		}, nil
	}

	prompt := a.presets["general_chat"]

	// 获取历史消息
	history := a.getChatHistory(sessionID, 5)

	messages := []Message{
		{Role: "system", Content: prompt},
	}
	messages = append(messages, history...)
	messages = append(messages, Message{Role: "user", Content: message})

	resp, err := a.minimax.Chat(messages)
	if err != nil {
		return &ChatResponse{
			Reply:  "抱歉，AI服务暂时不可用，请稍后再试。",
			Action: "ai_error",
		}, nil
	}

	return &ChatResponse{
		Reply:  resp,
		Action: "chat",
	}, nil
}

// saveMessage 保存消息
func (a *Agent) saveMessage(sessionID, userID uuid.UUID, role, content string) {
	msg := &model.ChatMessage{
		SessionID: sessionID,
		UserID:    userID,
		Role:      role,
		Content:   content,
		CreatedAt: time.Now(),
	}
	a.db.Create(msg)
}

// getChatHistory 获取聊天历史
func (a *Agent) getChatHistory(sessionID uuid.UUID, limit int) []Message {
	var messages []model.ChatMessage
	a.db.Where("session_id = ?", sessionID).Order("created_at DESC").Limit(limit).Find(&messages)

	result := make([]Message, 0)
	for i := len(messages) - 1; i >= 0; i-- {
		result = append(result, Message{
			Role:    messages[i].Role,
			Content: messages[i].Content,
		})
	}
	return result
}

// extractURL 从文本中提取URL
func extractURL(text string) string {
	re := regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`)
	if match := re.FindString(text); match != "" {
		return match
	}

	// 匹配短链接
	patterns := []string{
		`v\.douyin\.com/\w+`,
		`v\.kuaishou\.com/\w+`,
		`b23\.tv/\w+`,
		`xhslink\.com/\w+`,
		`youtu\.be/\w+`,
		`vm\.tiktok\.com/\w+`,
	}

	for _, pattern := range patterns {
		re = regexp.MustCompile(pattern)
		if match := re.FindString(text); match != "" {
			return "https://" + match
		}
	}

	return ""
}

// ChatResponse AI对话响应
type ChatResponse struct {
	Reply    string `json:"reply"`
	Action   string `json:"action"`
	VideoURL string `json:"video_url,omitempty"`
	CoverURL string `json:"cover_url,omitempty"`
	Title    string `json:"title,omitempty"`
	TaskID   string `json:"task_id,omitempty"`
}
