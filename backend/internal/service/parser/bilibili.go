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

// BilibiliParser B站解析器
type BilibiliParser struct{}

func (p *BilibiliParser) Platform() string {
	return "bilibili"
}

func (p *BilibiliParser) Match(url string) bool {
	patterns := []string{
		"bilibili.com",
		"b23.tv",
	}

	for _, pattern := range patterns {
		if strings.Contains(url, pattern) {
			return true
		}
	}
	return false
}

func (p *BilibiliParser) Parse(url string) (*ParseResult, error) {
	// 1. 获取重定向URL
	realURL, err := p.getRealURL(url)
	if err != nil {
		return &ParseResult{
			Success: false,
			Error:   "获取视频链接失败: " + err.Error(),
		}, nil
	}

	// 2. 提取BV号
	bvid := p.extractBVID(realURL)
	if bvid == "" {
		return &ParseResult{
			Success: false,
			Error:   "无法提取视频BV号",
		}, nil
	}

	// 3. 获取视频信息
	videoInfo, err := p.getVideoInfo(bvid)
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

func (p *BilibiliParser) getRealURL(url string) (string, error) {
	if !strings.Contains(url, "b23.tv") {
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

func (p *BilibiliParser) extractBVID(url string) string {
	re := regexp.MustCompile(`BV[a-zA-Z0-9]+`)
	return re.FindString(url)
}

func (p *BilibiliParser) getVideoInfo(bvid string) (*VideoInfo, error) {
	// 使用B站API获取视频信息
	apiURL := fmt.Sprintf("https://api.bilibili.com/x/web-interface/view?bvid=%s", bvid)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			Bvid     string `json:"bvid"`
			Cid      int    `json:"cid"`
			Title    string `json:"title"`
			Desc     string `json:"desc"`
			Duration int    `json:"duration"`
			Owner    struct {
				Name string `json:"name"`
			} `json:"owner"`
			Pic  string `json:"pic"`
			Stat struct {
				View int `json:"view"`
				Like int `json:"like"`
			} `json:"stat"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, errors.New("获取视频信息失败")
	}

	// 获取视频播放地址
	videoURL, _ := p.getVideoPlayURL(bvid, result.Data.Cid)

	return &VideoInfo{
		Platform: "bilibili",
		VideoID:  result.Data.Bvid,
		Title:    result.Data.Title,
		Author:   result.Data.Owner.Name,
		CoverURL: result.Data.Pic,
		VideoURL: videoURL,
		Duration: result.Data.Duration,
	}, nil
}

// getVideoPlayURL 获取视频播放地址
func (p *BilibiliParser) getVideoPlayURL(bvid string, cid int) (string, error) {
	apiURL := fmt.Sprintf("https://api.bilibili.com/x/player/playurl?bvid=%s&cid=%d&qn=80&fnval=16", bvid, cid)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			Dash struct {
				Video []struct {
					BaseURL string `json:"baseUrl"`
				} `json:"video"`
				Audio []struct {
					BaseURL string `json:"baseUrl"`
				} `json:"audio"`
			} `json:"dash"`
			Durl []struct {
				URL string `json:"url"`
			} `json:"durl"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", errors.New("获取播放地址失败")
	}

	// 优先使用Dash格式
	if len(result.Data.Dash.Video) > 0 {
		return result.Data.Dash.Video[0].BaseURL, nil
	}

	// 备用Durl格式
	if len(result.Data.Durl) > 0 {
		return result.Data.Durl[0].URL, nil
	}

	return "", errors.New("无可用播放地址")
}
