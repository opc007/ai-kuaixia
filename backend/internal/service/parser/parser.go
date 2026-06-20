package parser

import (
	"errors"
	"regexp"
	"strings"
)

// VideoInfo 视频信息
type VideoInfo struct {
	Platform  string `json:"platform"`
	VideoID   string `json:"video_id"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	CoverURL  string `json:"cover_url"`
	VideoURL  string `json:"video_url"`
	Duration  int    `json:"duration"`
	FileSize  int64  `json:"file_size"`
	Quality   string `json:"quality"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

// ParseResult 解析结果
type ParseResult struct {
	Success bool        `json:"success"`
	Data    *VideoInfo  `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Parser 解析器接口
type Parser interface {
	// Platform 平台名称
	Platform() string
	// Match 匹配URL
	Match(url string) bool
	// Parse 解析视频
	Parse(url string) (*ParseResult, error)
}

// ParserFactory 解析器工厂
type ParserFactory struct {
	parsers []Parser
}

func NewParserFactory() *ParserFactory {
	factory := &ParserFactory{
		parsers: make([]Parser, 0),
	}

	// 注册所有解析器
	factory.Register(&DouyinParser{})
	factory.Register(&KuaishouParser{})
	factory.Register(&BilibiliParser{})
	factory.Register(&XiaohongshuParser{})
	factory.Register(&YoutubeParser{})
	factory.Register(&TiktokParser{})
	factory.Register(&WeishiParser{})
	factory.Register(&XiguaParser{})
	
	// 通用解析器
	factory.Register(NewGenericParser())

	return factory
}

func (f *ParserFactory) Register(parser Parser) {
	f.parsers = append(f.parsers, parser)
}

// Parse 解析视频URL
func (f *ParserFactory) Parse(url string) (*ParseResult, error) {
	// 清理URL
	url = strings.TrimSpace(url)

	// 尝试从文本中提取URL
	extractedURL := extractURL(url)
	if extractedURL != "" {
		url = extractedURL
	}

	// 匹配解析器
	for _, parser := range f.parsers {
		if parser.Match(url) {
			result, err := parser.Parse(url)
			// 如果解析成功，直接返回
			if err == nil && result != nil && result.Success {
				return result, nil
			}
			// 如果是平台特定解析器失败，继续尝试下一个
			if parser.Platform() != "generic" {
				continue
			}
			// 如果通用解析器失败，尝试第三方解析
			if parser.Platform() == "generic" {
				thirdParty := NewThirdPartyParser()
				tpResult, tpErr := thirdParty.Parse(url)
				if tpErr == nil && tpResult != nil && tpResult.Success {
					return tpResult, nil
				}
				// 返回通用解析器的错误
				if result != nil {
					return result, nil
				}
			}
		}
	}

	return nil, errors.New("不支持的平台或链接格式错误")
}

// extractURL 从文本中智能提取URL
func extractURL(text string) string {
	text = strings.TrimSpace(text)

	// 如果本身就是URL，直接返回
	if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
		// 清理URL末尾的标点符号
		return cleanURL(text)
	}

	// 匹配常见URL格式（按优先级排序）
	patterns := []string{
		// 抖音
		`https?://v\.douyin\.com/\w+/?`,
		`https?://www\.douyin\.com/video/\d+`,
		`https?://www\.iesdouyin\.com/share/video/\d+`,
		// 快手
		`https?://v\.kuaishou\.com/\w+/?`,
		`https?://www\.kuaishou\.com/short-video/\w+`,
		`https?://www\.kuaishou\.com/f/\w+`,
		// B站
		`https?://b23\.tv/\w+/?`,
		`https?://www\.bilibili\.com/video/\w+`,
		`https?://m\.bilibili\.com/video/\w+`,
		// 小红书
		`https?://xhslink\.com/\w+/?`,
		`https?://www\.xiaohongshu\.com/explore/\w+`,
		`https?://www\.xiaohongshu\.com/discovery/item/\w+`,
		// YouTube
		`https?://youtu\.be/\w+/?`,
		`https?://www\.youtube\.com/watch\?v=\w+`,
		`https?://m\.youtube\.com/watch\?v=\w+`,
		`https?://www\.youtube\.com/shorts/\w+`,
		// TikTok
		`https?://vm\.tiktok\.com/\w+/?`,
		`https?://www\.tiktok\.com/@[\w.]+/video/\d+`,
		`https?://m\.tiktok\.com/v/\d+`,
		// 微视
		`https?://isee\.weishi\.qq\.com/\w+`,
		`https?://video\.weishi\.qq\.com/\w+`,
		// 西瓜视频
		`https?://www\.ixigua\.com/\d+`,
		`https?://m\.ixigua\.com/video/\d+`,
		// 通用HTTP/HTTPS链接（最后匹配）
		`https?://[^\s<>"{}|\\^` + "`" + `\]\[）】\s]+`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindString(text); match != "" {
			return cleanURL(match)
		}
	}

	// 尝试匹配不带协议的短链接
	shortLinkPatterns := []string{
		`v\.douyin\.com/\w+`,
		`v\.kuaishou\.com/\w+`,
		`b23\.tv/\w+`,
		`xhslink\.com/\w+`,
		`youtu\.be/\w+`,
		`vm\.tiktok\.com/\w+`,
	}

	for _, pattern := range shortLinkPatterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindString(text); match != "" {
			return "https://" + match
		}
	}

	return ""
}

// cleanURL 清理URL末尾的标点符号
func cleanURL(url string) string {
	// 移除末尾的标点符号（这些可能是文本的一部分，不是URL的一部分）
	trailingChars := []string{
		"。", "，", "！", "？", "；", "：", "、", "）", "】", "」", "』", "》",
		".", ",", "!", "?", ";", ":", ")", "]", "}", "'",
	}

	for {
		trimmed := false
		for _, char := range trailingChars {
			if strings.HasSuffix(url, char) {
				url = strings.TrimSuffix(url, char)
				trimmed = true
			}
		}
		if !trimmed {
			break
		}
	}

	return url
}

// GetSupportedPlatforms 获取支持的平台列表
func (f *ParserFactory) GetSupportedPlatforms() []map[string]string {
	platforms := make([]map[string]string, 0)
	for _, parser := range f.parsers {
		platforms = append(platforms, map[string]string{
			"name": parser.Platform(),
		})
	}
	return platforms
}
