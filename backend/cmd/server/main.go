package main

import (
	"log"
	"os"

	"github.com/aikuaixia/aikuaixia/internal/api/handler"
	"github.com/aikuaixia/aikuaixia/internal/api/middleware"
	"github.com/aikuaixia/aikuaixia/internal/model"
	"github.com/aikuaixia/aikuaixia/internal/pkg/config"
	"github.com/aikuaixia/aikuaixia/internal/pkg/seed"
	"github.com/aikuaixia/aikuaixia/internal/service/ai"
	"github.com/aikuaixia/aikuaixia/internal/service/parser"
	"github.com/aikuaixia/aikuaixia/internal/service/payment"
	"github.com/aikuaixia/aikuaixia/internal/service/user"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	// 加载配置
	cfg := config.Load()

	// 连接数据库（优先PostgreSQL，回退SQLite）
	var db *gorm.DB
	var err error

	// 检查是否使用SQLite
	dbDriver := os.Getenv("DB_DRIVER")
	if dbDriver == "sqlite" || os.Getenv("DB_HOST") == "" {
		// 使用SQLite
		dbPath := os.Getenv("DB_PATH")
		if dbPath == "" {
			dbPath = "aikuaixia.db"
		}
		db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
		if err != nil {
			log.Fatal("Failed to connect to SQLite:", err)
		}
		log.Println("Using SQLite database")
	} else {
		// 使用PostgreSQL
		dsn := "host=" + cfg.DBHost +
			" user=" + cfg.DBUser +
			" password=" + cfg.DBPassword +
			" dbname=" + cfg.DBName +
			" port=" + cfg.DBPort +
			" sslmode=disable"
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			log.Fatal("Failed to connect to PostgreSQL:", err)
		}
		log.Println("Using PostgreSQL database")
	}

	// 自动迁移
	err = db.AutoMigrate(
		&model.User{},
		&model.PasswordResetToken{},
		&model.UserCredits{},
		&model.CreditTransaction{},
		&model.RechargeOrder{},
		&model.ConsumeOrder{},
		&model.DownloadTask{},
		&model.ChatSession{},
		&model.ChatMessage{},
		&model.AIServiceLog{},
		&model.Platform{},
		&model.AIPreset{},
		&model.CreditPackage{},
	)
	if err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	// 初始化服务
	userSvc := user.NewUserService(db, cfg)
	creditSvc := user.NewCreditService(db)
	parserSvc := parser.NewParserFactory()
	paySvc := payment.NewPaymentService(db, cfg, creditSvc)
	agentSvc := ai.NewAgent(db, cfg, parserSvc, creditSvc)

	// 初始化处理器
	authHandler := handler.NewAuthHandler(userSvc)
	parseHandler := handler.NewParseHandler(parserSvc)
	paymentHandler := handler.NewPaymentHandler(paySvc, creditSvc)
	aiHandler := handler.NewAIHandler(agentSvc)
	downloadHandler := handler.NewDownloadHandler()
	downloadV2Handler := handler.NewDownloadV2Handler(creditSvc)
	adminHandler := handler.NewAdminHandler(db, creditSvc, paySvc)

	// 启动时种子数据（套餐 / 平台 / AI 预设）
	if err := seed.Run(db); err != nil {
		log.Printf("seed warn: %v", err)
	}

	// 初始化Gin
	r := gin.Default()

	// 跨域
	r.Use(corsMiddleware())

	// 公开接口
	public := r.Group("/api/v1")
	{
		public.POST("/auth/register", authHandler.Register)
		public.POST("/auth/login", authHandler.Login)
		public.POST("/auth/send-code", authHandler.SendCode)
		public.POST("/auth/login-by-code", authHandler.LoginByCode)
		public.POST("/auth/forgot-password", authHandler.ForgotPassword)
		public.POST("/auth/reset-password", authHandler.ResetPassword)
		public.POST("/auth/oauth/:provider", authHandler.OAuthLogin)
		public.GET("/platforms", parseHandler.GetPlatforms)
		public.GET("/packages", paymentHandler.GetPackages)
		public.GET("/payment/methods", paymentHandler.GetPaymentMethods)
		public.POST("/parse", parseHandler.Parse)
		public.GET("/download", downloadHandler.DownloadVideo)
		// download/v2 需要鉴权
	}

	// 支付回调
	r.POST("/api/v1/payment/alipay/notify", paymentHandler.HandleAlipayNotify)
	r.POST("/api/v1/payment/wechat/notify", paymentHandler.HandleWechatNotify)

	// 需要认证的接口
	authorized := r.Group("/api/v1")
	authorized.Use(middleware.AuthMiddleware(userSvc))
	{
		authorized.GET("/user/profile", authHandler.GetProfile)
		authorized.POST("/auth/change-password", authHandler.ChangePassword)
		authorized.POST("/payment/create", paymentHandler.CreateRechargeOrder)
		authorized.GET("/payment/status", paymentHandler.QueryOrderStatus)
		authorized.GET("/credits/transactions", paymentHandler.GetCreditTransactions)
		authorized.GET("/recharge/orders", paymentHandler.GetRechargeOrders)
		authorized.GET("/download/v2", downloadV2Handler.DownloadVideo)
		authorized.POST("/ai/chat", aiHandler.Chat)
		authorized.GET("/ai/history", aiHandler.GetHistory)
	}

	
	// 管理端路由（用户名以 admin_ 开头）
	admin := r.Group("/api/v1/admin")
	admin.Use(middleware.AdminMiddleware(userSvc))
	{
		admin.GET("/dashboard", adminHandler.DashboardStats)
		admin.GET("/users", adminHandler.ListUsers)
		admin.POST("/users/status", adminHandler.ToggleUserStatus)
		admin.GET("/orders", adminHandler.ListOrders)
		admin.GET("/platforms", adminHandler.ListPlatforms)
		admin.POST("/platforms", adminHandler.UpdatePlatform)
		admin.GET("/ai-presets", adminHandler.ListAIPresets)
		admin.POST("/ai-presets", adminHandler.UpdateAIPreset)
		admin.POST("/packages", adminHandler.UpdateCreditPackage)
	}

	// 静态文件：网页端 / 下载客户端
	r.Static("/web-user", "./web-user")
	r.Static("/web", "./web")
	r.StaticFile("/favicon.ico", "./web-user/favicon.ico")

// 启动服务
	port := os.Getenv("PORT")
	if port == "" {
		port = cfg.Port
	}

	log.Printf("AI快侠服务启动，端口: %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
