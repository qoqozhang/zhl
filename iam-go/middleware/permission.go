package middleware

import (
	"context"
	"net/http"

	"iam-go/pkg/logger"

	"github.com/gin-gonic/gin"
)

type PermissionMatcher interface {
	MatchPermissionKey(ctx context.Context, fullPath, method string) (string, error)
}

type RolePermissionChecker interface {
	HasPermission(ctx context.Context, roleKey, permissionKey string) (bool, error)
}

// PermissionMiddleware 细粒度 API 权限校验
// 流程:
//  1. 从 Gin 路由拿到 FullPath（如 /v1/users/:id）
//  2. 从 permission 表匹配 (path, http_method) → permission_key
//  3. 从 JWT claims.role 拿 role_key
//  4. 查 sys_role_permission → 有无绑定
//  5. admin 角色自动放行
//  6. 路由没配置 permission（没匹配到）→ 放行（避免遗漏路由全部 403）
func PermissionMiddleware(matcher PermissionMatcher, checker RolePermissionChecker) gin.HandlerFunc {
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

		fullPath := c.FullPath()
		method := c.Request.Method

		permKey, err := matcher.MatchPermissionKey(c.Request.Context(), fullPath, method)
		if err != nil {
			logger.Warn().Err(err).Str("path", fullPath).Str("method", method).Msg("match permission key failed, skip check")
			c.Next()
			return
		}
		if permKey == "" {
			c.Next()
			return
		}

		ok, err := checker.HasPermission(c.Request.Context(), claims.Role, permKey)
		if err != nil {
			logger.Warn().Err(err).Str("role", claims.Role).Str("perm", permKey).Msg("check permission failed, deny")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code": 40300, "message": "forbidden: no permission",
			})
			return
		}
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code": 40300, "message": "forbidden: no permission",
			})
			return
		}
		c.Next()
	}
}