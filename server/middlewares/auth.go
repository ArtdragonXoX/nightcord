package middlewares

import (
	"net/http"
	"nightcord-server/internal/conf"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware 用于验证认证 token，检查请求头 Authorization 是否匹配
func AuthMiddleware(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token != conf.Conf.Server.Token {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的认证token"})
		c.Abort()
		return
	}
	c.Next()
}
