package parser

import (
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// KuaishouParser 快手解析器
type KuaishouParser struct{}

func (p *KuaishouParser) Platform() string {
	return "kuaishou"
}

func (p *KuaishouParser) Match(url string) bool {
	patterns := []string{
		"kuaishou.com",
		"gifshow.com",
		"v.kuaishou.com",
	}

	for _, pattern := range patterns {
		if strings.Contains(url, pattern) {
			return true
		}
	}
	return false
}

func (p *KuaishouParser) Parse(url string) (*ParseResult, error) {
	// 1. 获取重定向后的真实URL
	realURL, err := p.getRealURL(url)
	if err != nil {
		return &ParseResult{
			Success: false,
			Error:   "获取视频链接失败: " + err.Error(),
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

func (p *KuaishouParser) getRealURL(url string) (string, error) {
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

func (p *KuaishouParser) extractVideoID(url string) string {
	// 匹配 /short-video/xxx 或 /photo/xxx 格式
	re := regexp.MustCompile(`/short-video/(\w+)`)
	if match := re.FindStringSubmatch(url); len(match) > 1 {
		return match[1]
	}

	re = regexp.MustCompile(`/photo/(\w+)`)
	if match := re.FindStringSubmatch(url); len(match) > 1 {
		return match[1]
	}

	return ""
}

func (p *KuaishouParser) getVideoInfo(url string) (*VideoInfo, error) {
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

	// 从HTML中提取视频信息（简化处理）
	videoInfo := &VideoInfo{
		Platform: "kuaishou",
	}

	// 提取标题
	titleRe := regexp.MustCompile(`<title>(.*?)</title>`)
	if match := titleRe.FindStringSubmatch(html); len(match) > 1 {
		videoInfo.Title = strings.TrimSpace(match[1])
	}

	// 提取视频URL（需要更复杂的解析）
	videoRe := regexp.MustCompile(`"playUrl":"(.*?)"`)
	if match := videoRe.FindStringSubmatch(html); len(match) > 1 {
		videoURL := strings.ReplaceAll(match[1], "\\u002F", "/")
		videoInfo.VideoURL = videoURL
	}

	// 提取封面
	coverRe := regexp.MustCompile(`"coverUrl":"(.*?)"`)
	if match := coverRe.FindStringSubmatch(html); len(match) > 1 {
		coverURL := strings.ReplaceAll(match[1], "\\u002F", "/")
		videoInfo.CoverURL = coverURL
	}

	if videoInfo.VideoURL == "" {
		return nil, errors.New("无法解析视频链接")
	}

	return videoInfo, nil
}
