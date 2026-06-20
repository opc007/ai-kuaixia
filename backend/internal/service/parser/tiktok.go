package parser

import (
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// TiktokParser TikTok解析器
type TiktokParser struct{}

func (p *TiktokParser) Platform() string {
	return "tiktok"
}

func (p *TiktokParser) Match(url string) bool {
	patterns := []string{
		"tiktok.com",
		"vm.tiktok.com",
	}

	for _, pattern := range patterns {
		if strings.Contains(url, pattern) {
			return true
		}
	}
	return false
}

func (p *TiktokParser) Parse(url string) (*ParseResult, error) {
	// 1. 获取重定向URL
	realURL, err := p.getRealURL(url)
	if err != nil {
		return &ParseResult{
			Success: false,
			Error:   "获取链接失败: " + err.Error(),
		}, nil
	}

	// 2. 提取视频ID
	videoID := p.extractVideoID(realURL)
	if videoID == "" {
		return &ParseResult{
			Success: false,
			Error:   "无法提取视频ID",
		}, nil
	}

	// 3. 获取视频信息
	videoInfo, err := p.getVideoInfo(realURL)
	if err != nil {
		return &ParseResult{
			Success: false,
			Error:   "获取视频信息失败: " + err.Error(),
		}, nil
	}

	return &ParseResult{
		Success: true,
		Data:    videoInfo,
	}, nil
}

func (p *TiktokParser) getRealURL(url string) (string, error) {
	if !strings.Contains(url, "vm.tiktok.com") {
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

func (p *TiktokParser) extractVideoID(url string) string {
	re := regexp.MustCompile(`/video/(\d+)`)
	if match := re.FindStringSubmatch(url); len(match) > 1 {
		return match[1]
	}
	return ""
}

func (p *TiktokParser) getVideoInfo(url string) (*VideoInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15")

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
		Platform: "tiktok",
	}

	// 提取标题
	titleRe := regexp.MustCompile(`<title>(.*?)</title>`)
	if match := titleRe.FindStringSubmatch(html); len(match) > 1 {
		videoInfo.Title = strings.TrimSpace(match[1])
	}

	// 提取视频URL
	videoRe := regexp.MustCompile(`"playAddr":"(.*?)"`)
	if match := videoRe.FindStringSubmatch(html); len(match) > 1 {
		videoURL := strings.ReplaceAll(match[1], "\\u002F", "/")
		videoInfo.VideoURL = videoURL
	}

	// 提取封面
	coverRe := regexp.MustCompile(`"cover":"(.*?)"`)
	if match := coverRe.FindStringSubmatch(html); len(match) > 1 {
		coverURL := strings.ReplaceAll(match[1], "\\u002F", "/")
		videoInfo.CoverURL = coverURL
	}

	if videoInfo.VideoURL == "" {
		return nil, errors.New("无法解析视频链接")
	}

	return videoInfo, nil
}
