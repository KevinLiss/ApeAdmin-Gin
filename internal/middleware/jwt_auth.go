package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/dal"
	"apeadmin-gin/internal/pkg/response"
)

// JWTAuth JWT 认证中间件
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(401, "请先登录"))
			return
		}
		tokenString := strings.TrimPrefix(auth, "Bearer ")
		claims, err := core.ParseToken(tokenString)
		if err != nil || claims.Type != "access" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(401, "登录已过期，请重新登录"))
			return
		}

		userID, err := core.GetUserIDFromClaims(claims)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(401, "登录状态异常"))
			return
		}

		// 黑名单校验
		cfg := core.GetConfig()
		if cfg.JWT.BlacklistEnabled {
			tokenStore := core.GetTokenStore()
			if tokenStore != nil {
				if revoked, _ := tokenStore.IsRevoked(c.Request.Context(), claims.ID); revoked {
					c.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(401, "登录已失效"))
					return
				}
			}
		}

		// 查库验证用户状态 + 版本号校验
		user, err := dal.GetUserByID(userID)
		if err != nil || user.Status != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(401, "用户不存在或已禁用"))
			return
		}
		if user.TokenVersion != claims.TokenVersion {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(401, "登录状态已失效，请重新登录"))
			return
		}

		c.Set("user", user)
		c.Set("user_id", user.ID)
		c.Set("username", user.Username)
		c.Set("jti", claims.ID)
		c.Next()
	}
}
