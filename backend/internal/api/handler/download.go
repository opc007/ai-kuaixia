package handler

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type DownloadHandler struct{}

func NewDownloadHandler() *DownloadHandler {
	return &DownloadHandler{}
}

// DownloadVideo 下载视频（代理下载，解决跨域和防盗链问题）
func (h *DownloadHandler) DownloadVideo(c *gin.Context) {
	videoURL := c.Query("url")
	if videoURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少视频链接"})
		return
	}

	platform := c.Query("platform")

	// 创建带cookie jar的客户端
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		Timeout: 60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}

	// 对于抖音，先访问分享页面获取cookie
	if platform == "douyin" || strings.Contains(videoURL, "douyinvod.com") || strings.Contains(videoURL, "douyin.com") {
		h.prepareDouyinCookies(client)
	}

	// 校验 URL（禁止非 http/https）
	u, err := url.Parse(videoURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的视频链接"})
		return
	}

	// 创建下载请求
	req, err := http.NewRequest("GET", videoURL, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的视频链接"})
		return
	}

	// 转发客户端 Range（支持断点/播放跳转）
	if rangeHdr := c.GetHeader("Range"); rangeHdr != "" {
		req.Header.Set("Range", rangeHdr)
	}

	// 设置请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	// 根据平台设置Referer
	switch platform {
	case "douyin":
		req.Header.Set("Referer", "https://www.douyin.com/")
	case "kuaishou":
		req.Header.Set("Referer", "https://www.kuaishou.com/")
	case "bilibili":
		req.Header.Set("Referer", "https://www.bilibili.com/")
	case "xiaohongshu":
		req.Header.Set("Referer", "https://www.xiaohongshu.com/")
	case "youtube":
		req.Header.Set("Referer", "https://www.youtube.com/")
	case "tiktok":
		req.Header.Set("Referer", "https://www.tiktok.com/")
	default:
		if strings.HasPrefix(videoURL, "http") {
			idx := strings.Index(videoURL[8:], "/")
			if idx > 0 {
				req.Header.Set("Referer", videoURL[:8+idx+1])
			}
		}
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "下载失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	// 支持上游返回 200/206（已支持分片）
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		c.JSON(resp.StatusCode, gin.H{"error": "下载失败，状态码: " + resp.Status})
		return
	}

	// 设置响应头：按上游返回头转发关键头
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "video/mp4"
	}

	c.Header("Content-Type", contentType)
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		c.Header("Content-Range", cr)
	}
	if ar := resp.Header.Get("Accept-Ranges"); ar != "" {
		c.Header("Accept-Ranges", ar)
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		c.Header("Content-Length", cl)
	}
	// 尝试从上游 Content-Disposition 或 URL 推断文件名
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		c.Header("Content-Disposition", cd)
	} else {
		c.Header("Content-Disposition", "attachment; filename=video.mp4")
	}
	// CORS 由全局中间件处理，不在此重复设置

	// 返回上游状态码（支持 206）
	c.Status(resp.StatusCode)

	// 流式传输
	io.Copy(c.Writer, resp.Body)
}

// prepareDouyinCookies 预先获取抖音cookie
func (h *DownloadHandler) prepareDouyinCookies(client *http.Client) {
	// 访问抖音分享页面获取ttwid等cookie
	urls := []string{
		"https://www.douyin.com/",
		"https://www.iesdouyin.com/",
	}
	
	for _, url := range urls {
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
		
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}
}
