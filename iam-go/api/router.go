package api

import (
	"iam-go/api/v1"
	"iam-go/middleware"
	"iam-go/pkg/auditlog"
	jwtutil "iam-go/pkg/jwt"
	"iam-go/pkg/session"
	"iam-go/services"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type RegisterRoutesOption struct {
	RedisClient  *redis.Client
	GormDB       *gorm.DB
	JWTManager   *jwtutil.Manager
	SessionStore *session.Store
	CasbinSvc    interface{}
}

// RegisterRoutes 注册所有业务路由
func RegisterRoutes(r *gin.Engine, opt *RegisterRoutesOption) {
	var casbinEnforcer middleware.CasbinEnforcer
	if opt.CasbinSvc != nil {
		if cs, ok := opt.CasbinSvc.(interface {
			Enforce(role, path, method string) (bool, error)
		}); ok {
			casbinEnforcer = cs
		}
	}
	// 全局中间件
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.Logger())

	// 操作审计日志中间件（需要 GORM）
	if opt != nil && opt.GormDB != nil {
		r.Use(auditlog.AuditMiddleware(opt.GormDB))
	}

	// 健康检查（不记审计日志）
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	// 构造限流器
	var authorizeLimiter middleware.RateLimiter = middleware.NewMemoryRateLimiter(1000, time.Second)
	if opt != nil && opt.RedisClient != nil {
		authorizeLimiter = middleware.NewRedisRateLimiter(opt.RedisClient, 1000, time.Second, "rl:authorize:")
	}

	// ============ /v1 路由组 ============
	v1Group := r.Group("/v1")
	{
		v1Group.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "healthy"})
		})

		if opt == nil || opt.GormDB == nil || opt.JWTManager == nil {
			return
		}

		// ============ /oauth2 OAuth 2.0 路由组 ============
		oauthAPI := NewOAuthAPI(opt.GormDB, opt.JWTManager, opt.SessionStore)
		oauthGroup := r.Group("/oauth2")
		{
			oauthGroup.GET("/authorize", middleware.RateLimitMiddleware(authorizeLimiter), oauthAPI.Authorize)
			oauthGroup.POST("/token", oauthAPI.Token)
			oauthGroup.GET("/userinfo", middleware.AuthMiddlewareOptional(opt.JWTManager, opt.SessionStore), oauthAPI.UserInfo)
		}

		userSvc := services.NewUserService(opt.GormDB)
		userAPI := v1.NewUserAPI(userSvc, opt.JWTManager, opt.SessionStore)

		deptSvc := services.NewDepartmentService(opt.GormDB)
		deptAPI := v1.NewDepartmentAPI(deptSvc, opt.JWTManager)

		roleSvc := services.NewRoleService(opt.GormDB)
		rpSvc := services.NewRolePermissionService(opt.GormDB)
		if cs, ok := opt.CasbinSvc.(services.CasbinSyncer); ok {
			rpSvc.WithSyncer(cs)
		}
		roleAPI := v1.NewRoleAPI(roleSvc, rpSvc, opt.JWTManager)

		permSvc := services.NewPermissionService(opt.GormDB)
		permAPI := v1.NewPermissionAPI(permSvc, opt.JWTManager)

		ocSvc := services.NewOAuthClientService(opt.GormDB)
		ocAPI := v1.NewOAuthClientAPI(ocSvc)

		// === 公开接口 ===
		v1Group.POST("/users/login", userAPI.Login)
		v1Group.POST("/users/refresh", userAPI.Refresh)
		v1Group.POST("/users/logout", userAPI.Logout)

		// === 需要认证 + Casbin 细粒度鉴权 ===
		authMiddlewares := []gin.HandlerFunc{
			middleware.AuthMiddleware(opt.JWTManager, opt.SessionStore),
		}
		if casbinEnforcer != nil {
			authMiddlewares = append(authMiddlewares, middleware.CasbinPermissionMiddleware(casbinEnforcer))
		}
		authGroup := v1Group.Group("", authMiddlewares...)
		{
			authGroup.GET("/users/me", userAPI.Me)
			authGroup.GET("/users/:id", userAPI.Get)
			authGroup.PUT("/users/:id", userAPI.Update)

			authGroup.GET("/departments", deptAPI.List)
			authGroup.GET("/departments/:id", deptAPI.Get)

			authGroup.GET("/roles", roleAPI.List)
			authGroup.GET("/roles/:id", roleAPI.Get)

			authGroup.GET("/permissions", permAPI.List)
			authGroup.GET("/permissions/:id", permAPI.Get)
		}

		// === 需要 admin 角色 ===
		adminGroup := v1Group.Group("",
			middleware.AuthMiddleware(opt.JWTManager, opt.SessionStore),
			middleware.RequireRole("admin"),
		)
		{
			adminGroup.POST("/users", userAPI.Create)
			adminGroup.GET("/users", userAPI.List)
			adminGroup.DELETE("/users/:id", userAPI.Delete)

			adminGroup.POST("/departments", deptAPI.Create)
			adminGroup.PUT("/departments/:id", deptAPI.Update)
			adminGroup.DELETE("/departments/:id", deptAPI.Delete)

			adminGroup.POST("/roles", roleAPI.Create)
			adminGroup.PUT("/roles/:id", roleAPI.Update)
			adminGroup.DELETE("/roles/:id", roleAPI.Delete)
			adminGroup.PUT("/roles/:id/permissions", roleAPI.UpdatePermissions)

			adminGroup.POST("/permissions", permAPI.Create)
			adminGroup.PUT("/permissions/:id", permAPI.Update)
			adminGroup.DELETE("/permissions/:id", permAPI.Delete)

			adminGroup.POST("/oauth/clients", ocAPI.Create)
			adminGroup.GET("/oauth/clients", ocAPI.List)
			adminGroup.GET("/oauth/clients/:id", ocAPI.Get)
			adminGroup.PUT("/oauth/clients/:id", ocAPI.Update)
			adminGroup.DELETE("/oauth/clients/:id", ocAPI.Delete)
			adminGroup.POST("/oauth/clients/:id/reset-secret", ocAPI.ResetSecret)
		}
	}
}