package parser

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// YoutubeParser YouTube解析器
type YoutubeParser struct{}

func (p *YoutubeParser) Platform() string {
	return "youtube"
}

func (p *YoutubeParser) Match(url string) bool {
	patterns := []string{
		"youtube.com",
		"youtu.be",
		"m.youtube.com",
	}

	for _, pattern := range patterns {
		if strings.Contains(url, pattern) {
			return true
		}
	}
	return false
}

func (p *YoutubeParser) Parse(url string) (*ParseResult, error) {
	// 提取视频ID
	videoID := p.extractVideoID(url)
	if videoID == "" {
		return &ParseResult{
			Success: false,
			Error:   "无法提取视频ID",
		}, nil
	}

	// 获取视频信息
	videoInfo, err := p.getVideoInfo(videoID)
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

func (p *YoutubeParser) extractVideoID(url string) string {
	// 匹配 youtube.com/watch?v=xxx
	re := regexp.MustCompile(`[?&]v=([a-zA-Z0-9_-]{11})`)
	if match := re.FindStringSubmatch(url); len(match) > 1 {
		return match[1]
	}

	// 匹配 youtu.be/xxx
	re = regexp.MustCompile(`youtu\.be/([a-zA-Z0-9_-]{11})`)
	if match := re.FindStringSubmatch(url); len(match) > 1 {
		return match[1]
	}

	// 匹配 youtube.com/embed/xxx
	re = regexp.MustCompile(`youtube\.com/embed/([a-zA-Z0-9_-]{11})`)
	if match := re.FindStringSubmatch(url); len(match) > 1 {
		return match[1]
	}

	return ""
}

func (p *YoutubeParser) getVideoInfo(videoID string) (*VideoInfo, error) {
	// 使用YouTube oEmbed API获取视频信息
	apiURL := "https://www.youtube.com/oembed?url=https://www.youtube.com/watch?v=" + videoID + "&format=json"

	// 创建Transport，支持代理
	transport := &http.Transport{
		Proxy: p.getProxy(),
	}

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		// 如果代理失败，尝试直连
		if transport.Proxy != nil {
			client.Transport = &http.Transport{}
			resp, err = client.Do(req)
			if err != nil {
				return nil, errors.New("YouTube服务暂时不可用，请稍后重试")
			}
		} else {
			return nil, errors.New("YouTube服务暂时不可用，请稍后重试")
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, errors.New("获取视频信息失败")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Title     string `json:"title"`
		Author    string `json:"author_name"`
		Thumbnail string `json:"thumbnail_url"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &VideoInfo{
		Platform: "youtube",
		VideoID:  videoID,
		Title:    result.Title,
		Author:   result.Author,
		CoverURL: result.Thumbnail,
		Width:    result.Width,
		Height:   result.Height,
	}, nil
}

// getProxy 从环境变量获取代理配置
func (p *YoutubeParser) getProxy() func(*http.Request) (*url.URL, error) {
	// 优先使用HTTPS_PROXY
	proxyURL := os.Getenv("HTTPS_PROXY")
	if proxyURL == "" {
		proxyURL = os.Getenv("https_proxy")
	}
	if proxyURL == "" {
		proxyURL = os.Getenv("HTTP_PROXY")
	}
	if proxyURL == "" {
		proxyURL = os.Getenv("http_proxy")
	}
	if proxyURL == "" {
		// 尝试使用本地代理
		proxyURL = os.Getenv("YOUTUBE_PROXY")
	}

	if proxyURL == "" {
		return nil
	}

	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil
	}

	return http.ProxyURL(u)
}
