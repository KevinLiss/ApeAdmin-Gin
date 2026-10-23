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
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.ErrorCtx(c, 401, "auth.not_logged_in", nil))
			return
		}
		tokenString := strings.TrimPrefix(auth, "Bearer ")
		claims, err := core.ParseToken(tokenString)
		if err != nil || claims.Type != "access" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.ErrorCtx(c, 401, "auth.login_expired", nil))
			return
		}

		userID, err := core.GetUserIDFromClaims(claims)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.ErrorCtx(c, 401, "auth.login_abnormal", nil))
			return
		}

		// 黑名单校验
		cfg := core.GetConfig()
		if cfg.JWT.BlacklistEnabled {
			tokenStore := core.GetTokenStore()
			if tokenStore != nil {
				if revoked, _ := tokenStore.IsRevoked(c.Request.Context(), claims.ID); revoked {
					c.AbortWithStatusJSON(http.StatusUnauthorized, response.ErrorCtx(c, 401, "auth.login_invalid", nil))
					return
				}
			}
		}

		// 查库验证用户状态 + 版本号校验
		user, err := dal.GetUserByID(userID)
		if err != nil || user.Status != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.ErrorCtx(c, 401, "auth.user_disabled", nil))
			return
		}
		if user.TokenVersion != claims.TokenVersion {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.ErrorCtx(c, 401, "auth.login_invalid", nil))
			return
		}

		c.Set("user", user)
		c.Set("user_id", user.ID)
		c.Set("username", user.Username)
		c.Set("jti", claims.ID)
		// 登记在线会话（在线用户列表 / 强制下线用）
		core.TrackOnline(user.ID, user.Username, claims.ID, c.ClientIP(), c.GetHeader("User-Agent"))
		c.Next()
	}
}
