package config

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	// 服务器配置
	Port string
	Mode string // debug/release

	// 数据库配置
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// Redis配置
	RedisHost     string
	RedisPort     string
	RedisPassword string

	// JWT配置
	JWTSecret string
	JWTExpire int // 小时

	// MiniMax AI配置
	MiniMaxAPIKey  string
	MiniMaxModel   string

	// 支付宝配置
	AlipayAppID      string
	AlipayPrivateKey string
	AlipayPublicKey  string
	AlipayNotifyURL  string

	// 微信支付配置
	WechatAppID     string
	WechatMchID     string
	WechatAPIKey    string
	WechatNotifyURL string

	// 积分配置
	RegisterCredits int // 注册赠送积分
	DownloadCredits int // 下载扣积分
	ChatCredits     int // AI对话扣积分

	// 文件存储
	StorageType string // local/minio/oss
	StoragePath string

	// 邮件 SMTP（可选；为空时验证码走 debug 直接返回）
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string // 发件人邮箱
	SMTPFromName string // 发件人显示名
	SMTPSiteName string // 站点名（用于邮件标题/正文）
}

func Load() *Config {
	loadDotEnv()
	return &Config{
		Port: getEnv("PORT", "8080"),
		Mode: getEnv("MODE", "debug"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "aikuaixia"),

		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		JWTSecret: getEnv("JWT_SECRET", "aikuaixia-secret-key-2026"),
		JWTExpire: getEnvInt("JWT_EXPIRE", 72), // 3天

		MiniMaxAPIKey: getEnv("MINIMAX_API_KEY", ""),
		MiniMaxModel:  getEnv("MINIMAX_MODEL", "MiniMax-M3"),

		AlipayAppID:      getEnv("ALIPAY_APP_ID", ""),
		AlipayPrivateKey: getEnv("ALIPAY_PRIVATE_KEY", ""),
		AlipayPublicKey:  getEnv("ALIPAY_PUBLIC_KEY", ""),
		AlipayNotifyURL:  getEnv("ALIPAY_NOTIFY_URL", ""),

		WechatAppID:     getEnv("WECHAT_APP_ID", ""),
		WechatMchID:     getEnv("WECHAT_MCH_ID", ""),
		WechatAPIKey:    getEnv("WECHAT_API_KEY", ""),
		WechatNotifyURL: getEnv("WECHAT_NOTIFY_URL", ""),

		RegisterCredits: getEnvInt("REGISTER_CREDITS", 10),
		DownloadCredits: getEnvInt("DOWNLOAD_CREDITS", 1),
		ChatCredits:     getEnvInt("CHAT_CREDITS", 1),

		StorageType: getEnv("STORAGE_TYPE", "local"),
		StoragePath: getEnv("STORAGE_PATH", "./storage"),

		SMTPHost:     getEnv("SMTP_HOST", ""),
		SMTPPort:     getEnvInt("SMTP_PORT", 465),
		SMTPUser:     getEnv("SMTP_USER", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     getEnv("SMTP_FROM", ""),
		SMTPFromName: getEnv("SMTP_FROM_NAME", "AI快侠"),
		SMTPSiteName: getEnv("SMTP_SITE_NAME", "AI快侠"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// loadDotEnv 轻量 .env 加载器（无第三方依赖）
// 支持 KEY=VALUE 与 export KEY=VALUE 格式，# 开头为注释
func loadDotEnv() {
	path := os.Getenv("ENV_FILE")
	if path == "" {
		path = ".env"
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		val = strings.Trim(val, `"\'`)
		if key == "" {
			continue
		}
		// 不覆盖显式设置的 env
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, val)
	}
	if err := scanner.Err(); err != nil {
		log.Printf("read .env error: %v", err)
	}
}
