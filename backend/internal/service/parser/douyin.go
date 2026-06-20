package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// DouyinParser 抖音解析器
type DouyinParser struct{}

func (p *DouyinParser) Platform() string {
	return "douyin"
}

func (p *DouyinParser) Match(url string) bool {
	patterns := []string{
		"douyin.com",
		"iesdouyin.com",
		"v.douyin.com",
	}

	for _, pattern := range patterns {
		if strings.Contains(url, pattern) {
			return true
		}
	}
	return false
}

func (p *DouyinParser) Parse(url string) (*ParseResult, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil // 允许重定向
		},
	}

	// 1. 跟踪重定向获取真实URL
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &ParseResult{Success: false, Error: "创建请求失败"}, nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return &ParseResult{Success: false, Error: "请求失败: " + err.Error()}, nil
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()

	// 2. 提取视频ID
	videoID := p.extractVideoID(finalURL)
	if videoID == "" {
		// 从页面内容提取
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		
		re := regexp.MustCompile(`/video/(\d+)`)
		matches := re.FindStringSubmatch(bodyStr)
		if len(matches) > 1 {
			videoID = matches[1]
		}
		
		if videoID == "" {
			re2 := regexp.MustCompile(`/note/(\d+)`)
			matches2 := re2.FindStringSubmatch(bodyStr)
			if len(matches2) > 1 {
				videoID = matches2[1]
			}
		}
	}

	if videoID == "" {
		return &ParseResult{Success: false, Error: "无法提取视频ID"}, nil
	}

	// 3. 尝试多种方式获取视频信息
	videoInfo, err := p.getVideoInfo(client, videoID)
	if err != nil {
		return &ParseResult{Success: false, Error: "获取视频信息失败: " + err.Error()}, nil
	}

	return &ParseResult{Success: true, Data: videoInfo}, nil
}

func (p *DouyinParser) extractVideoID(url string) string {
	// 匹配 /video/1234567890 格式
	re := regexp.MustCompile(`/video/(\d+)`)
	if match := re.FindStringSubmatch(url); len(match) > 1 {
		return match[1]
	}

	// 匹配 /note/1234567890 格式（图文）
	re = regexp.MustCompile(`/note/(\d+)`)
	if match := re.FindStringSubmatch(url); len(match) > 1 {
		return match[1]
	}

	return ""
}

func (p *DouyinParser) getVideoInfo(client *http.Client, videoID string) (*VideoInfo, error) {
	// 方法1: 尝试新版API
	videoInfo, err := p.tryNewAPI(client, videoID)
	if err == nil && videoInfo != nil {
		return videoInfo, nil
	}

	// 方法2: 尝试旧版API
	videoInfo, err = p.tryOldAPI(client, videoID)
	if err == nil && videoInfo != nil {
		return videoInfo, nil
	}

	// 方法3: 从移动端SSR页面提取
	videoInfo, err = p.tryMobileSSR(client, videoID)
	if err == nil && videoInfo != nil {
		return videoInfo, nil
	}

	return nil, errors.New("所有解析方式均失败")
}

// tryNewAPI 尝试新版API
func (p *DouyinParser) tryNewAPI(client *http.Client, videoID string) (*VideoInfo, error) {
	apiURL := fmt.Sprintf("https://www.douyin.com/aweme/v1/web/aweme/detail/?aweme_id=%s&aid=1128&version_name=23.5.0&device_platform=android&os_version=2333", videoID)

	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 11; Pixel 5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.91 Mobile Safari/537.36")
	req.Header.Set("Referer", "https://www.douyin.com/")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	detail, ok := result["aweme_detail"].(map[string]interface{})
	if !ok || detail == nil {
		return nil, errors.New("aweme_detail为空")
	}

	return p.parseVideoDetail(detail, videoID)
}

// tryOldAPI 尝试旧版API
func (p *DouyinParser) tryOldAPI(client *http.Client, videoID string) (*VideoInfo, error) {
	apiURL := fmt.Sprintf("https://www.iesdouyin.com/web/api/v2/aweme/iteminfo/?item_ids=%s", videoID)

	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15")
	req.Header.Set("Referer", "https://www.douyin.com/")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	items, ok := result["item_list"].([]interface{})
	if !ok || len(items) == 0 {
		return nil, errors.New("item_list为空")
	}

	item := items[0].(map[string]interface{})
	return p.parseVideoDetail(item, videoID)
}

// tryMobileSSR 从移动端SSR页面提取
func (p *DouyinParser) tryMobileSSR(client *http.Client, videoID string) (*VideoInfo, error) {
	mobileURL := fmt.Sprintf("https://www.iesdouyin.com/share/video/%s/", videoID)

	req, _ := http.NewRequest("GET", mobileURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15")
	req.Header.Set("Accept", "text/html")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	// 提取 _ROUTER_DATA
	re := regexp.MustCompile(`_ROUTER_DATA\s*=\s*(\{[\s\S]+?\})\s*</script>`)
	matches := re.FindStringSubmatch(html)
	if len(matches) < 1 {
		return nil, errors.New("未找到_ROUTER_DATA")
	}

	var routerData map[string]interface{}
	if err := json.Unmarshal([]byte(matches[1]), &routerData); err != nil {
		return nil, err
	}

	loaderData, ok := routerData["loaderData"].(map[string]interface{})
	if !ok {
		return nil, errors.New("loaderData为空")
	}

	pageData, ok := loaderData["video_(id)/page"].(map[string]interface{})
	if !ok {
		return nil, errors.New("pageData为空")
	}

	videoInfoRes, ok := pageData["videoInfoRes"].(map[string]interface{})
	if !ok {
		return nil, errors.New("videoInfoRes为空")
	}

	itemList, ok := videoInfoRes["item_list"].([]interface{})
	if !ok || len(itemList) == 0 {
		// 检查过滤原因
		if filterList, ok := videoInfoRes["filter_list"].([]interface{}); ok && len(filterList) > 0 {
			filter := filterList[0].(map[string]interface{})
			reason, _ := filter["filter_reason"].(string)
			return nil, fmt.Errorf("视频被过滤: %s", reason)
		}
		return nil, errors.New("item_list为空")
	}

	item := itemList[0].(map[string]interface{})
	return p.parseVideoDetail(item, videoID)
}

// parseVideoDetail 解析视频详情
func (p *DouyinParser) parseVideoDetail(item map[string]interface{}, videoID string) (*VideoInfo, error) {
	desc, _ := item["desc"].(string)

	authorName := ""
	if author, ok := item["author"].(map[string]interface{}); ok {
		authorName, _ = author["nickname"].(string)
	}

	videoURL := ""
	coverURL := ""
	width := 0
	height := 0
	duration := 0

	if video, ok := item["video"].(map[string]interface{}); ok {
		if playAddr, ok := video["play_addr"].(map[string]interface{}); ok {
			if urlList, ok := playAddr["url_list"].([]interface{}); ok && len(urlList) > 0 {
				videoURL, _ = urlList[0].(string)
				// 保持playwm（带水印）URL，play（无水印）需要更高权限会返回403
			}
		}

		if cover, ok := video["cover"].(map[string]interface{}); ok {
			if urlList, ok := cover["url_list"].([]interface{}); ok && len(urlList) > 0 {
				coverURL, _ = urlList[0].(string)
			}
		}

		if w, ok := video["width"].(float64); ok {
			width = int(w)
		}
		if h, ok := video["height"].(float64); ok {
			height = int(h)
		}
		if d, ok := video["duration"].(float64); ok {
			duration = int(d) / 1000
		}
	}

	if videoURL == "" {
		return nil, errors.New("无法获取视频下载地址")
	}

	return &VideoInfo{
		Platform: "douyin",
		VideoID:  videoID,
		Title:    desc,
		Author:   authorName,
		CoverURL: coverURL,
		VideoURL: videoURL,
		Duration: duration,
		Width:    width,
		Height:   height,
	}, nil
}
