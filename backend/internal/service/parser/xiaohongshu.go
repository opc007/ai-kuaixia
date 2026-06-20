package parser

import (
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// XiaohongshuParser 小红书解析器
type XiaohongshuParser struct{}

func (p *XiaohongshuParser) Platform() string {
	return "xiaohongshu"
}

func (p *XiaohongshuParser) Match(url string) bool {
	patterns := []string{
		"xiaohongshu.com",
		"xhslink.com",
		"xhs.cn",
	}

	for _, pattern := range patterns {
		if strings.Contains(url, pattern) {
			return true
		}
	}
	return false
}

func (p *XiaohongshuParser) Parse(url string) (*ParseResult, error) {
	// 1. 获取重定向URL
	realURL, err := p.getRealURL(url)
	if err != nil {
		return &ParseResult{
			Success: false,
			Error:   "获取链接失败: " + err.Error(),
		}, nil
	}

	// 2. 提取笔记ID
	noteID := p.extractNoteID(realURL)
	if noteID == "" {
		return &ParseResult{
			Success: false,
			Error:   "无法提取笔记ID",
		}, nil
	}

	// 3. 获取笔记信息
	videoInfo, err := p.getNoteInfo(realURL)
	if err != nil {
		return &ParseResult{
			Success: false,
			Error:   "获取笔记信息失败: " + err.Error(),
		}, nil
	}

	return &ParseResult{
		Success: true,
		Data:    videoInfo,
	}, nil
}

func (p *XiaohongshuParser) getRealURL(url string) (string, error) {
	if !strings.Contains(url, "xhslink.com") {
		return url, nil
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 302 || resp.StatusCode == 301 {
		location := resp.Header.Get("Location")
		if location != "" {
			return location, nil
		}
	}

	return url, nil
}

func (p *XiaohongshuParser) extractNoteID(url string) string {
	re := regexp.MustCompile(`/explore/([a-f0-9]+)`)
	if match := re.FindStringSubmatch(url); len(match) > 1 {
		return match[1]
	}

	re = regexp.MustCompile(`/discovery/item/([a-f0-9]+)`)
	if match := re.FindStringSubmatch(url); len(match) > 1 {
		return match[1]
	}

	return ""
}

func (p *XiaohongshuParser) getNoteInfo(url string) (*VideoInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	html := string(body)

	videoInfo := &VideoInfo{
		Platform: "xiaohongshu",
	}

	// 提取标题
	titleRe := regexp.MustCompile(`<title>(.*?)</title>`)
	if match := titleRe.FindStringSubmatch(html); len(match) > 1 {
		videoInfo.Title = strings.TrimSpace(match[1])
	}

	// 提取视频URL
	videoRe := regexp.MustCompile(`"originVideoKey":"(.*?)"`)
	if match := videoRe.FindStringSubmatch(html); len(match) > 1 {
		videoInfo.VideoURL = "https://sns-video-bd.xhscdn.com/" + match[1]
	}

	// 提取封面
	coverRe := regexp.MustCompile(`"coverUrl":"(.*?)"`)
	if match := coverRe.FindStringSubmatch(html); len(match) > 1 {
		videoInfo.CoverURL = strings.ReplaceAll(match[1], "\\u002F", "/")
	}

	if videoInfo.VideoURL == "" {
		return nil, errors.New("无法解析视频链接，可能不是视频笔记")
	}

	return videoInfo, nil
}
