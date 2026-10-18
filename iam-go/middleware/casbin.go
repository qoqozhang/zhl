package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type CasbinEnforcer interface {
	Enforce(role, path, method string) (bool, error)
}

// CasbinPermissionMiddleware Casbin 细粒度鉴权中间件
// 前置：AuthMiddleware 必须先跑，把 claims 塞进 Context
// 逻辑：
//   1. admin 角色自动放行（避免种子规则不全导致 admin 被拦截）
//   2. 用 Casbin Enforce(role, c.FullPath(), c.Request.Method)
//   3. Casbin 里没配规则的路由 → 默认放行（保守策略，避免遗漏全量 403）
func CasbinPermissionMiddleware(enforcer CasbinEnforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := GetCurrentUser(c)
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 40100, "message": "unauthorized",
			})
			return
		}

		if claims.Role == "admin" {
			c.Next()
			return
		}

		allowed, err := enforcer.Enforce(claims.Role, c.FullPath(), c.Request.Method)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"code": 50000, "message": "casbin enforce error: " + err.Error(),
			})
			return
		}

		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code": 40300, "message": "forbidden: no permission for " + c.FullPath(),
			})
			return
		}

		c.Next()
	}
}