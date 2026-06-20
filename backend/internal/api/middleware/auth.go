package middleware

import (
	"net/http"
	"strings"

	"github.com/aikuaixia/aikuaixia/internal/service/user"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func AuthMiddleware(userSvc *user.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从Header获取Token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未提供认证信息"})
			c.Abort()
			return
		}

		// 解析Token
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		userID, err := userSvc.ParseToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "认证信息无效"})
			c.Abort()
			return
		}

		// 将用户ID存入上下文
		c.Set("user_id", userID)
		c.Next()
	}
}


// AdminMiddleware 管理端鉴权：复用用户 JWT，校验用户名以 admin_ 开头
func AdminMiddleware(userSvc *user.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未提供认证信息"})
			c.Abort()
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		userID, err := userSvc.ParseToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "认证信息无效"})
			c.Abort()
			return
		}

		uid, _ := uuid.Parse(userID)
		u, err := userSvc.GetUserByID(uid)
		if err != nil || !strings.HasPrefix(u.Username, "admin_") {
			c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限（用户名以 admin_ 开头）"})
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Next()
	}
}
