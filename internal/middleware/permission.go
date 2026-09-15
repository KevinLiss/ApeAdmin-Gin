package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gin-apeadmin/internal/model"
	"gin-apeadmin/internal/pkg/response"
	"gin-apeadmin/internal/service"
)

// RequirePermission 权限校验中间件
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userVal, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(401, "请先登录"))
			return
		}
		u := userVal.(*model.SysUser)

		// 超管直接放行（读 is_super_admin 字段，不比对用户名）
		if u.IsSuperAdmin {
			c.Next()
			return
		}

		// 收集用户权限集合
		perms := service.GetUserPermissions(u.ID)
		if !perms[permission] && !perms["*"] {
			c.AbortWithStatusJSON(http.StatusForbidden, response.Error(403, "无权限: "+permission))
			return
		}
		c.Next()
	}
}
