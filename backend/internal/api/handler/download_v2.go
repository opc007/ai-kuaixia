package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aikuaixia/aikuaixia/internal/service/user"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type DownloadV2Handler struct {
	tempDir   string
	creditSvc *user.CreditService
}

func NewDownloadV2Handler(creditSvc *user.CreditService) *DownloadV2Handler {
	tempDir := filepath.Join(os.TempDir(), "aikuaixia_downloads")
	os.MkdirAll(tempDir, 0755)
	return &DownloadV2Handler{tempDir: tempDir, creditSvc: creditSvc}
}

// DownloadVideo 下载视频
// 需要登录；下载前扣 1 积分，下载失败时退还。
// 优先直接 HTTP 代理（适用 .m4s/.mp4 直链，自动加 Referer）
// 失败再回退到 yt-dlp（适用抖音/快手等需解析的分享链接）
func (h *DownloadV2Handler) DownloadVideo(c *gin.Context) {
	shareURL := c.Query("url")
	if shareURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少视频链接"})
		return
	}
	platform := c.Query("platform")
	title := c.Query("title")

	// 当前用户
	uidStr, _ := c.Get("user_id")
	uid, err := uuid.Parse(fmt.Sprintf("%v", uidStr))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	// 预扣 1 积分
	_, err = h.creditSvc.DeductCredits(uid, 1, "video_download", nil)
	if err != nil {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "积分不足，请充值"})
		return
	}

	filename := fmt.Sprintf("v%d%s", time.Now().UnixNano(), guessExt(shareURL, platform))
	outputPath := filepath.Join(h.tempDir, filename)

	var dlErr error
	if needsYtDlp(shareURL, platform) {
		// 分享短链（抖音/快手等）直接走 yt-dlp，HTTP 代理只会拿到 HTML 页面
		if !h.tryYtDlp(shareURL, outputPath) {
			_ = os.Remove(outputPath)
			_ = h.creditSvc.AddCredits(uid, 1, "refund_download", nil, "下载失败退还")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":    "下载失败: 无法解析视频，请确认链接有效",
				"refunded": true,
			})
			return
		}
	} else {
		// CDN 直链优先 HTTP 代理
		referer := pickReferer(platform, shareURL)
		dlErr = h.directDownload(shareURL, outputPath, referer)
		if dlErr != nil || !isValidVideoFile(outputPath) {
			_ = os.Remove(outputPath)
			if !h.tryYtDlp(shareURL, outputPath) {
				_ = os.Remove(outputPath)
				_ = h.creditSvc.AddCredits(uid, 1, "refund_download", nil, "下载失败退还")
				errMsg := "下载失败"
				if dlErr != nil {
					errMsg = fmt.Sprintf("下载失败: %s", dlErr.Error())
				}
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":    errMsg,
					"refunded": true,
				})
				return
			}
		}
	}

	// 检查文件是否存在且为有效视频
	if _, err := os.Stat(outputPath); os.IsNotExist(err) || !isValidVideoFile(outputPath) {
		// yt-dlp 可能使用了带扩展名的文件名，在目录中查找最新有效视频
		pattern := filepath.Join(h.tempDir, "v*")
		matches, _ := filepath.Glob(pattern)
		var latest string
		var latestTime time.Time
		for _, m := range matches {
			if !isValidVideoFile(m) {
				continue
			}
			if info, err := os.Stat(m); err == nil && info.ModTime().After(latestTime) {
				latest = m
				latestTime = info.ModTime()
			}
		}
		if latest == "" {
			_ = h.creditSvc.AddCredits(uid, 1, "refund_download", nil, "文件未找到退还")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "下载文件未找到或无效"})
			return
		}
		outputPath = latest
	}

	fileInfo, err := os.Stat(outputPath)
	if err != nil {
		_ = h.creditSvc.AddCredits(uid, 1, "refund_download", nil, "无法读取文件退还")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法读取文件"})
		return
	}

	// 设置响应头
	safeTitle := sanitizeFilename(title)
	if safeTitle == "" {
		safeTitle = "video"
	}
	c.Header("Content-Type", "video/mp4")
	c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.mp4\"", safeTitle))
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Cache-Control", "no-cache")

	file, err := os.Open(outputPath)
	if err != nil {
		_ = h.creditSvc.AddCredits(uid, 1, "refund_download", nil, "无法打开文件退还")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法打开文件"})
		return
	}
	defer file.Close()
	defer os.Remove(outputPath)

	io.Copy(c.Writer, file)
}

// tryYtDlp 用 yt-dlp 下载
func (h *DownloadV2Handler) tryYtDlp(shareURL, outputPath string) bool {
	ytdlp := findYtDlp()
	cmd := exec.Command(ytdlp,
		"-f", "best[ext=mp4]/best[ext=webm]/best",
		"--no-check-certificates",
		"--no-warnings",
		"--no-playlist",
		"--no-overwrites",
		"-o", outputPath,
		shareURL,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("[download-v2] yt-dlp failed: %v output=%s\n", err, string(output))
		return false
	}
	if !isValidVideoFile(outputPath) {
		fmt.Printf("[download-v2] yt-dlp output invalid: %s\n", string(output))
		return false
	}
	return true
}

func findYtDlp() string {
	candidates := []string{
		"yt-dlp",
		"/opt/homebrew/bin/yt-dlp",
		"/usr/local/bin/yt-dlp",
	}
	for _, c := range candidates {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return "yt-dlp"
}

// directDownload 直接 HTTP 下载
func (h *DownloadV2Handler) directDownload(url, outputPath, referer string) error {
	client := &http.Client{
		Timeout: 5 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1")
	req.Header.Set("Accept", "*/*")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

// needsYtDlp 判断是否为分享短链（需 yt-dlp 解析，HTTP 直下会得到 HTML）
func needsYtDlp(url, platform string) bool {
	lower := strings.ToLower(url)
	shareHosts := []string{
		"v.douyin.com", "v.kuaishou.com", "b23.tv", "xhslink.com",
		"vm.tiktok.com", "vt.tiktok.com", "youtu.be",
	}
	for _, h := range shareHosts {
		if strings.Contains(lower, h) {
			return true
		}
	}
	// 平台页面链接（非 CDN 直链）
	if strings.Contains(lower, "douyin.com/video") ||
		strings.Contains(lower, "kuaishou.com/short-video") ||
		strings.Contains(lower, "xiaohongshu.com/explore") {
		return true
	}
	// 已知 CDN 直链特征
	if strings.Contains(lower, ".m4s") || strings.Contains(lower, ".mp4") ||
		strings.Contains(lower, "bilivideo.com") || strings.Contains(lower, ".webm") {
		return false
	}
	switch platform {
	case "douyin", "kuaishou", "xiaohongshu", "tiktok", "youtube":
		return true
	}
	return false
}

// isValidVideoFile 检查下载结果是否为有效视频（避免 HTML 页面被当作视频返回）
func isValidVideoFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 12)
	n, _ := f.Read(buf)
	if n < 4 {
		return false
	}
	// HTML 页面
	if strings.HasPrefix(string(buf[:min(n, 8)]), "<!") ||
		strings.HasPrefix(string(buf[:min(n, 5)]), "<html") {
		return false
	}
	// MP4/MOV: ....ftyp
	if n >= 8 && string(buf[4:8]) == "ftyp" {
		return true
	}
	// WebM: 0x1A45DFA3
	if buf[0] == 0x1A && buf[1] == 0x45 && buf[2] == 0xDF && buf[3] == 0xA3 {
		return true
	}
	// FLV
	if n >= 3 && string(buf[:3]) == "FLV" {
		return true
	}
	// 文件太小大概率不是视频
	info, err := os.Stat(path)
	if err != nil || info.Size() < 10*1024 {
		return false
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func guessExt(url, platform string) string {
	lower := strings.ToLower(url)
	switch {
	case strings.Contains(lower, ".m4s"):
		return ".m4s"
	case strings.Contains(lower, ".mp4"):
		return ".mp4"
	case strings.Contains(lower, ".webm"):
		return ".webm"
	case strings.Contains(lower, ".flv"):
		return ".flv"
	}
	return ".mp4"
}

func pickReferer(platform, url string) string {
	lower := strings.ToLower(url)
	switch {
	case strings.Contains(lower, "bilivideo.com"), strings.Contains(lower, "bilibili.com"), platform == "bilibili":
		return "https://www.bilibili.com/"
	case strings.Contains(lower, "douyin.com"), platform == "douyin":
		return "https://www.douyin.com/"
	case strings.Contains(lower, "kuaishou.com"), platform == "kuaishou":
		return "https://www.kuaishou.com/"
	case strings.Contains(lower, "xiaohongshu.com"), platform == "xiaohongshu":
		return "https://www.xiaohongshu.com/"
	case strings.Contains(lower, "youtube.com"), strings.Contains(lower, "youtu.be"), platform == "youtube":
		return "https://www.youtube.com/"
	case strings.Contains(lower, "tiktok.com"), platform == "tiktok":
		return "https://www.tiktok.com/"
	}
	return ""
}

func sanitizeFilename(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	bad := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", "\n", "\r", "\t"}
	for _, b := range bad {
		s = strings.ReplaceAll(s, b, "_")
	}
	if len([]rune(s)) > 80 {
		s = string([]rune(s)[:80])
	}
	return s
}
