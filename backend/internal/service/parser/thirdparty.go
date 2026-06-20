package parser

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// ThirdPartyParser 第三方解析API
type ThirdPartyParser struct {
	client *http.Client
}

func NewThirdPartyParser() *ThirdPartyParser {
	return &ThirdPartyParser{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *ThirdPartyParser) Platform() string {
	return "thirdparty"
}

func (p *ThirdPartyParser) Match(url string) bool {
	// 匹配任何HTTP/HTTPS链接
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

func (p *ThirdPartyParser) Parse(targetURL string) (*ParseResult, error) {
	// 尝试多个第三方解析服务
	services := []func(string) (*VideoInfo, error){
		p.tryCobaltAPI,
		p.trySaveFromAPI,
		p.tryDirectExtraction,
	}

	for _, service := range services {
		videoInfo, err := service(targetURL)
		if err == nil && videoInfo != nil && videoInfo.VideoURL != "" {
			return &ParseResult{
				Success: true,
				Data:    videoInfo,
			}, nil
		}
	}

	return &ParseResult{
		Success: false,
		Error:   "该页面使用JavaScript动态加载视频，当前版本暂不支持此类网站。建议使用录屏工具或等待后续版本更新。",
	}, nil
}

// tryCobaltAPI 尝试Cobalt API（开源视频下载服务）
func (p *ThirdPartyParser) tryCobaltAPI(targetURL string) (*VideoInfo, error) {
	// Cobalt API endpoint (可以自建或使用公共实例)
	cobaltURL := "https://api.cobalt.tools/api/json"
	
	payload := map[string]string{
		"url":    targetURL,
		"vCodec": "h264",
		"vQuality": "720",
		"aFormat": "mp3",
	}
	
	jsonData, _ := json.Marshal(payload)
	
	req, err := http.NewRequest("POST", cobaltURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cobalt API返回状态码: %d", resp.StatusCode)
	}
	
	body, _ := io.ReadAll(resp.Body)
	
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	
	// 检查是否有下载链接
	if url, ok := result["url"].(string); ok && url != "" {
		return &VideoInfo{
			Platform: "网页视频",
			Title:    "视频",
			VideoURL: url,
		}, nil
	}
	
	return nil, fmt.Errorf("cobalt未返回下载链接")
}

// trySaveFromAPI 尝试SaveFrom风格的解析
func (p *ThirdPartyParser) trySaveFromAPI(targetURL string) (*VideoInfo, error) {
	// 这里可以集成SaveFrom或其他解析服务的API
	// 由于这些服务通常需要付费或有使用限制，这里返回nil
	return nil, fmt.Errorf("saveFrom API未配置")
}

// tryDirectExtraction 尝试直接提取页面中的视频
func (p *ThirdPartyParser) tryDirectExtraction(targetURL string) (*VideoInfo, error) {
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	
	// 提取视频URL
	videoURL := ""
	
	// 方法1: og:video
	patterns := []string{
		`<meta[^>]*property=["']og:video["'][^>]*content=["']([^"']+)["']`,
		`<meta[^>]*property=["']og:video:url["'][^>]*content=["']([^"']+)["']`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(html); len(matches) > 1 {
			videoURL = matches[1]
			break
		}
	}
	
	// 方法2: video标签
	if videoURL == "" {
		videoPatterns := []string{
			`<video[^>]*src=["']([^"']+)["']`,
			`<source[^>]*src=["']([^"']+\.mp4[^"']*)["']`,
		}
		
		for _, pattern := range videoPatterns {
			re := regexp.MustCompile(pattern)
			if matches := re.FindStringSubmatch(html); len(matches) > 1 {
				videoURL = matches[1]
				break
			}
		}
	}
	
	// 方法3: JavaScript中的视频链接
	if videoURL == "" {
		jsPatterns := []string{
			`"videoUrl"\s*:\s*"([^"]+)"`,
			`"video_url"\s*:\s*"([^"]+)"`,
			`"url"\s*:\s*"([^"]+\.mp4[^"]*)"`,
		}
		
		for _, pattern := range jsPatterns {
			re := regexp.MustCompile(pattern)
			if matches := re.FindStringSubmatch(html); len(matches) > 1 {
				videoURL = strings.ReplaceAll(matches[1], `\/`, "/")
				break
			}
		}
	}
	
	if videoURL == "" {
		return nil, fmt.Errorf("未找到视频链接")
	}
	
	// 提取标题
	title := ""
	titlePatterns := []string{
		`<meta[^>]*property=["']og:title["'][^>]*content=["']([^"']+)["']`,
		`<title[^>]*>([^<]+)</title>`,
	}
	
	for _, pattern := range titlePatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(html); len(matches) > 1 {
			title = strings.TrimSpace(matches[1])
			break
		}
	}
	
	if title == "" {
		title = "网页视频"
	}
	
	return &VideoInfo{
		Platform: "网页视频",
		Title:    title,
		VideoURL: videoURL,
	}, nil
}
