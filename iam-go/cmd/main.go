package main

import (
	"context"
	"gorm.io/gorm"
	"iam-go/pkg/session"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	jwtutil "iam-go/pkg/jwt"
	"iam-go/pkg/logger"
	"iam-go/services"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"iam-go/api"
	"iam-go/configs"
	"iam-go/model"
	dbmysql "iam-go/pkg/database/mysql"
	dbredis "iam-go/pkg/database/redis"
)

func main() {
	// 0. 初始化日志（最优先，确保后续都能用）
	logger.Init("info", "console", "stdout")

	// 1. 加载配置（硬编码示例，后续替换为 yaml/env 加载）
	cfg := configs.Config{
		Port: "8080",
		DB: configs.DBConfig{
			Host:              "127.0.0.1",
			Port:              3306,
			User:              "root",
			Password:          "123456",
			Database:          "iam",
			MaxOpenConns:      100,
			MaxIdleConns:      10,
			LogLevel:          2,
			EnableAutoMigrate: true,
		},
		Redis: configs.RedisConfig{
			Enable: true,
			Host:   "127.0.0.1",
			Port:   6379,
			DB:     0,
		},
		JWT: configs.JWTConfig{
			Secret:            "iam-go-dev-secret-please-change-in-production",
			ExpireMinutes:     30,
			RefreshExpireDays: 7,
			Issuer:            "iam-go",
		},
	}

	// 2. 初始化 GORM MySQL
	dbPool, err := dbmysql.New(&cfg.DB)
	if err != nil {
		logger.Fatal().Err(err).Msg("init mysql failed")
	}
	defer func() {
		if err := dbPool.Close(); err != nil {
			logger.Error().Err(err).Msg("close mysql pool error")
		} else {
			logger.Info().Msg("mysql pool closed gracefully")
		}
	}()
	logger.Info().
		Str("host", cfg.DB.Host).
		Int("port", cfg.DB.Port).
		Str("db", cfg.DB.Database).
		Int("max_open", cfg.DB.MaxOpenConns).
		Int("max_idle", cfg.DB.MaxIdleConns).
		Msg("mysql connected")

	// 3. 自动迁移
	if cfg.DB.EnableAutoMigrate {
		if err := model.Migrate(dbPool.GORM()); err != nil {
			logger.Fatal().Err(err).Msg("auto migrate failed")
		}
		logger.Info().Msg("auto migrate done")
	}

	// 3.1 初始化默认 admin 用户
	initDefaultAdmin(dbPool.GORM())

	// 3.2 初始化默认系统角色
	initDefaultRoles(dbPool.GORM())

	// 3.3 初始化 admin 用户 ↔ admin 角色绑定
	initDefaultAdminRoleBinding(dbPool.GORM())

	// 3.4 初始化默认权限资源
	initDefaultPermissions(dbPool.GORM())

	// 3.5 admin 角色自动绑定全部权限
	initDefaultAdminRolePermissions(dbPool.GORM())

	// 3.6 默认 OAuth 客户端
	initDefaultOAuthClient(dbPool.GORM())

	// 4. 初始化 Redis
	var rawRedis *redis.Client
	if cfg.Redis.Enable {
		if err := cfg.Redis.ValidateAndApplyDefaults(); err != nil {
			logger.Warn().Err(err).Msg("redis config invalid, fallback to in-memory")
		} else if redisClient, err := dbredis.New(&dbredis.Options{
			Addr:         cfg.Redis.Addr(),
			Password:     cfg.Redis.Password,
			DB:           cfg.Redis.DB,
			DialTimeout:  time.Duration(cfg.Redis.DialTimeout) * time.Second,
			ReadTimeout:  time.Duration(cfg.Redis.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(cfg.Redis.WriteTimeout) * time.Second,
			PoolSize:     cfg.Redis.PoolSize,
			MinIdleConns: cfg.Redis.MinIdleConns,
		}); err != nil {
			logger.Warn().Err(err).Msg("init redis failed, fallback to in-memory rate limiter")
		} else {
			rawRedis = redisClient.Raw()
			logger.Info().
				Str("addr", cfg.Redis.Addr()).
				Int("db", cfg.Redis.DB).
				Msg("redis connected")
		}
	}

	// 5. 初始化 JWT Manager
	jwtMgr, err := jwtutil.NewManager(&cfg.JWT, rawRedis)
	if err != nil {
		logger.Fatal().Err(err).Msg("init jwt manager failed")
	}
	logger.Info().
		Int("access_min", cfg.JWT.ExpireMinutes).
		Int("refresh_days", cfg.JWT.RefreshExpireDays).
		Bool("redis", rawRedis != nil).
		Msg("jwt manager ready")

	// 5. 初始化 Session Store（Redis）
	var sessionStore *session.Store
	if rawRedis != nil {
		sessionStore = session.NewStore(rawRedis)
		logger.Info().Msg("session store ready (redis, ttl=24h)")
	} else {
		logger.Warn().Msg("redis unavailable, session disabled (JWT Bearer only)")
	}

	// 5.5 初始化 Casbin 权限引擎
	var casbinSvc *services.CasbinService
	casbinSvc, err = services.NewCasbinService(dbPool.GORM(), "configs/casbin.conf")
	if err != nil {
		logger.Fatal().Err(err).Msg("init casbin enforcer failed")
	}
	if err := casbinSvc.EnsureStartupPolicy(context.Background()); err != nil {
		logger.Warn().Err(err).Msg("casbin startup policy load failed, will retry on permission change")
	} else {
		logger.Info().Msg("casbin permission engine ready (policy loaded)")
	}

	// 6. 启动 HTTP
	r := gin.Default()
	api.RegisterRoutes(r, &api.RegisterRoutesOption{
		RedisClient:  rawRedis,
		GormDB:       dbPool.GORM(),
		JWTManager:   jwtMgr,
		SessionStore: sessionStore,
		CasbinSvc:    casbinSvc,
	})

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}

	go func() {
		logger.Info().Str("addr", srv.Addr).Msg("server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("http server error")
		}
	}()

	// 7. 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info().Msg("received shutdown signal...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("http shutdown error")
	}
	logger.Info().Msg("shutdown complete")
}

// initDefaultAdmin 启动时确保有 admin 用户，没有就创建
// 用 username 查（role 字段由后续 sys_user_role 关联表提供）
func initDefaultAdmin(db *gorm.DB) {
	var count int64
	db.Model(&model.User{}).Where("username = ?", "admin").Count(&count)
	if count > 0 {
		logger.Info().Int("admin_count", int(count)).Msg("admin user already exists, skip init")
		return
	}

	hashedPwd, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		logger.Error().Err(err).Msg("init admin: hash password failed")
		return
	}

	admin := &model.User{
		Username:     "admin",
		PasswordHash: string(hashedPwd),
		RealName:     "系统管理员",
		Email:        "admin@iam.local",
		Status:       1,
	}
	if err := db.Create(admin).Error; err != nil {
		logger.Error().Err(err).Msg("init admin user failed")
		return
	}
	logger.Warn().
		Str("username", "admin").
		Str("password", "admin123").
		Msg("default admin user created | CHANGE IT IN PRODUCTION!")
}

// initDefaultRoles 启动时确保系统内置角色存在
// 幂等：已存在的 role_key 不会重复创建
func initDefaultRoles(db *gorm.DB) {
	roleSvc := services.NewRoleService(db)
	if err := roleSvc.EnsureDefaultRoles(context.Background()); err != nil {
		logger.Error().Err(err).Msg("init default roles failed")
		return
	}
	logger.Info().Msg("default system roles ensured (admin/user/viewer)")
}

// initDefaultAdminRoleBinding 启动时确保 admin 用户绑定了 admin 角色
// 幂等：已绑定则跳过
func initDefaultAdminRoleBinding(db *gorm.DB) {
	var adminUser model.User
	if err := db.Where("username = ?", "admin").First(&adminUser).Error; err != nil {
		logger.Warn().Err(err).Msg("admin user not found, skip role binding")
		return
	}

	var adminRole model.Role
	if err := db.Where("role_key = ?", "admin").First(&adminRole).Error; err != nil {
		logger.Warn().Err(err).Msg("admin role not found, skip role binding")
		return
	}

	urSvc := services.NewUserRoleService(db)
	if err := urSvc.Bind(context.Background(), adminUser.ID, adminRole.ID); err != nil {
		logger.Error().Err(err).Msg("bind admin user to admin role failed")
		return
	}
	logger.Info().
		Uint("user_id", adminUser.ID).
		Uint("role_id", adminRole.ID).
		Msg("admin user bound to admin role")
}

// initDefaultPermissions 启动时写入种子权限数据
// 幂等：已存在的 permission_key 不会重复创建
func initDefaultPermissions(db *gorm.DB) {
	permSvc := services.NewPermissionService(db)
	if err := permSvc.EnsureDefaultPermissions(context.Background()); err != nil {
		logger.Error().Err(err).Msg("init default permissions failed")
		return
	}
	logger.Info().Msg("default permissions ensured (24 items)")
}

// initDefaultAdminRolePermissions 启动时让 admin 角色绑定全部权限
// 幂等：已有的绑定不会重复创建
func initDefaultAdminRolePermissions(db *gorm.DB) {
	var adminRole model.Role
	if err := db.Where("role_key = ?", "admin").First(&adminRole).Error; err != nil {
		logger.Warn().Err(err).Msg("admin role not found, skip permission binding")
		return
	}

	var allPermIDs []uint
	db.Model(&model.Permission{}).Pluck("id", &allPermIDs)

	rpSvc := services.NewRolePermissionService(db)
	casbinSvc, err := services.NewCasbinService(db, "configs/casbin.conf")
	if err == nil {
		rpSvc.WithSyncer(casbinSvc)
	}

	if err := rpSvc.ReplacePermissions(context.Background(), adminRole.ID, allPermIDs); err != nil {
		logger.Error().Err(err).Msg("bind admin role permissions failed")
		return
	}
	logger.Info().
		Uint("role_id", adminRole.ID).
		Int("permissions", len(allPermIDs)).
		Msg("admin role permissions bound")
}

func initDefaultOAuthClient(db *gorm.DB) {
	svc := services.NewOAuthClientService(db)
	if err := svc.EnsureDefaultClient(context.Background()); err != nil {
		logger.Error().Err(err).Msg("init default oauth client failed")
		return
	}
	logger.Info().Msg("default oauth client ensured (client_id=default_web)")
}