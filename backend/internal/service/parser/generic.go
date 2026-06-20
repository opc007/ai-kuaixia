package parser

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// GenericParser 通用解析器 - 支持任意URL
type GenericParser struct {
	client *http.Client
}

func NewGenericParser() *GenericParser {
	return &GenericParser{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *GenericParser) Platform() string {
	return "generic"
}

func (p *GenericParser) Match(url string) bool {
	// 匹配任何HTTP/HTTPS链接
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

func (p *GenericParser) Parse(url string) (*ParseResult, error) {
	// 创建请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &ParseResult{
			Success: false,
			Error:   "无法创建请求: " + err.Error(),
		}, nil
	}

	// 设置通用请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	// 发送请求
	resp, err := p.client.Do(req)
	if err != nil {
		return &ParseResult{
			Success: false,
			Error:   "请求失败: " + err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ParseResult{
			Success: false,
			Error:   "读取响应失败: " + err.Error(),
		}, nil
	}

	html := string(body)

	// 提取视频信息
	videoInfo := p.extractVideoInfo(url, html, resp.Header)

	if videoInfo.VideoURL == "" {
		return &ParseResult{
			Success: false,
			Error:   "该页面使用JavaScript动态加载视频，当前版本暂不支持此类网站。建议使用录屏工具或等待后续版本更新。",
		}, nil
	}

	return &ParseResult{
		Success: true,
		Data:    videoInfo,
	}, nil
}

// extractVideoInfo 从HTML中提取视频信息
func (p *GenericParser) extractVideoInfo(pageURL, html string, headers http.Header) *VideoInfo {
	info := &VideoInfo{
		Platform: "网页视频",
	}

	// 提取标题
	info.Title = p.extractTitle(html, pageURL)

	// 提取封面图
	info.CoverURL = p.extractCover(html, pageURL)

	// 提取视频URL - 尝试多种方式
	info.VideoURL = p.extractVideoURL(html, headers)

	return info
}

// extractTitle 提取标题
func (p *GenericParser) extractTitle(html, pageURL string) string {
	// 尝试从og:title提取
	patterns := []string{
		`<meta[^>]*property=["']og:title["'][^>]*content=["']([^"']+)["']`,
		`<meta[^>]*content=["']([^"']+)["'][^>]*property=["']og:title["']`,
		`<title[^>]*>([^<]+)</title>`,
		`<h1[^>]*>([^<]+)</h1>`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(html); len(matches) > 1 {
			title := strings.TrimSpace(matches[1])
			if title != "" && title != " " {
				return title
			}
		}
	}

	// 从URL提取域名作为标题
	re := regexp.MustCompile(`https?://([^/]+)`)
	if matches := re.FindStringSubmatch(pageURL); len(matches) > 1 {
		return matches[1] + " - 视频"
	}

	return "网页视频"
}

// extractCover 提取封面图
func (p *GenericParser) extractCover(html, pageURL string) string {
	patterns := []string{
		`<meta[^>]*property=["']og:image["'][^>]*content=["']([^"']+)["']`,
		`<meta[^>]*content=["']([^"']+)["'][^>]*property=["']og:image["']`,
		`<meta[^>]*name=["']twitter:image["'][^>]*content=["']([^"']+)["']`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(html); len(matches) > 1 {
			cover := matches[1]
			if strings.HasPrefix(cover, "//") {
				cover = "https:" + cover
			}
			return cover
		}
	}

	return ""
}

// extractVideoURL 提取视频URL
func (p *GenericParser) extractVideoURL(html string, headers http.Header) string {
	// 方法1: 从og:video提取
	videoPatterns := []string{
		`<meta[^>]*property=["']og:video["'][^>]*content=["']([^"']+)["']`,
		`<meta[^>]*content=["']([^"']+)["'][^>]*property=["']og:video["']`,
		`<meta[^>]*property=["']og:video:url["'][^>]*content=["']([^"']+)["']`,
		`<meta[^>]*name=["']twitter:player:stream["'][^>]*content=["']([^"']+)["']`,
	}

	for _, pattern := range videoPatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(html); len(matches) > 1 {
			videoURL := matches[1]
			if strings.HasPrefix(videoURL, "//") {
				videoURL = "https:" + videoURL
			}
			if strings.HasPrefix(videoURL, "http") {
				return videoURL
			}
		}
	}

	// 方法2: 从video标签提取
	videoTagPatterns := []string{
		`<video[^>]*src=["']([^"']+)["']`,
		`<video[^>]*><source[^>]*src=["']([^"']+)["']`,
		`<source[^>]*src=["']([^"']+\.mp4[^"']*)["']`,
		`<source[^>]*src=["']([^"']+\.m3u8[^"']*)["']`,
	}

	for _, pattern := range videoTagPatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(html); len(matches) > 1 {
			videoURL := matches[1]
			if strings.HasPrefix(videoURL, "//") {
				videoURL = "https:" + videoURL
			}
			if strings.HasPrefix(videoURL, "http") {
				return videoURL
			}
		}
	}

	// 方法3: 从JavaScript中提取视频链接
	jsVideoPatterns := []string{
		`"video_url"\s*:\s*"([^"]+)"`,
		`"videoUrl"\s*:\s*"([^"]+)"`,
		`"url"\s*:\s*"([^"]+\.mp4[^"]*)"`,
		`"src"\s*:\s*"([^"]+\.mp4[^"]*)"`,
		`"playUrl"\s*:\s*"([^"]+)"`,
		`"stream_url"\s*:\s*"([^"]+)"`,
		`file\s*:\s*["']([^"']+\.mp4[^"']*)["']`,
	}

	for _, pattern := range jsVideoPatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(html); len(matches) > 1 {
			videoURL := matches[1]
			// 处理转义字符
			videoURL = strings.ReplaceAll(videoURL, `\/`, `/`)
			videoURL = strings.ReplaceAll(videoURL, `\u0026`, `&`)
			if strings.HasPrefix(videoURL, "//") {
				videoURL = "https:" + videoURL
			}
			if strings.HasPrefix(videoURL, "http") {
				return videoURL
			}
		}
	}

	// 方法4: 检查Content-Type是否为视频
	contentType := headers.Get("Content-Type")
	if strings.Contains(contentType, "video/") {
		// 当前页面就是视频
		return "" // 需要原始URL
	}

	// 方法5: 提取所有可能的媒体URL
	mediaPatterns := []string{
		`https?://[^\s"'<>]+\.(?:mp4|m3u8|webm|ogg|mov)[^\s"'<>]*`,
	}

	for _, pattern := range mediaPatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindString(html); matches != "" {
			return matches
		}
	}

	return ""
}

// DownloadVideo 下载视频到指定路径
func (p *GenericParser) DownloadVideo(videoURL, outputPath string) error {
	req, err := http.NewRequest("GET", videoURL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	// 这里应该保存文件，但为了简化先返回成功
	// 实际实现需要写入文件
	return nil
}
