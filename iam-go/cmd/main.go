package main

import (
	"context"
	"strings"

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
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
	"iam-go/api"
	"iam-go/configs"
	"iam-go/model"
	dbmysql "iam-go/pkg/database/mysql"
	dbredis "iam-go/pkg/database/redis"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "iam-go",
		Short: "IAM identity and access management service",
		Long:  "A lightweight IAM service providing authentication, authorization, and OAuth2 capabilities.",
		RunE:  runServer,
	}

	registerFlags(rootCmd)
	cobra.OnInitialize(initViper)

	if err := rootCmd.Execute(); err != nil {
		logger.Fatal().Err(err).Msg("command failed")
	}
}

func registerFlags(cmd *cobra.Command) {
	f := cmd.Flags()

	f.String("port", "8080", "HTTP server listen port")

	f.String("log-level", "info", "Log level: debug | info | warn | error")
	f.String("log-format", "console", "Log format: console | json")
	f.String("log-output", "stdout", "Log output: stdout | stderr | file")

	f.String("db-host", "127.0.0.1", "MySQL host")
	f.Int("db-port", 3306, "MySQL port")
	f.String("db-user", "root", "MySQL user")
	f.String("db-password", "123456", "MySQL password")
	f.String("db-name", "iam", "MySQL database name")
	f.Int("db-max-open-conns", 100, "MySQL max open connections")
	f.Int("db-max-idle-conns", 10, "MySQL max idle connections")
	f.Int("db-log-level", 2, "GORM log level: 0=Silent 1=Error 2=Warn 3=Info")
	f.Bool("db-auto-migrate", true, "Enable auto migrate on startup")

	f.Bool("redis-enable", true, "Enable Redis cache")
	f.String("redis-host", "127.0.0.1", "Redis host")
	f.Int("redis-port", 6379, "Redis port")
	f.String("redis-password", "", "Redis password")
	f.Int("redis-db", 0, "Redis DB index")

	f.String("jwt-secret", "iam-go-dev-secret-please-change-in-production", "JWT signing secret")
	f.Int("jwt-expire-minutes", 30, "Access token expire minutes")
	f.Int("jwt-refresh-expire-days", 7, "Refresh token expire days")
	f.String("jwt-issuer", "iam-go", "JWT issuer")
}

func initViper() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("configs/")
	viper.AddConfigPath("./configs")

	viper.AutomaticEnv()
	viper.SetEnvPrefix("IAM")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic(err)
		}
	}
}

func applyCobraFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			key := strings.ReplaceAll(f.Name, "-", ".")
			viper.Set(key, f.Value.String())
		}
	})
}

func runServer(cmd *cobra.Command, _ []string) error {
	applyCobraFlags(cmd)

	cfg := configs.Config{}
	if err := viper.Unmarshal(&cfg); err != nil {
		return err
	}

	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "console"
	}
	if cfg.Log.Output == "" {
		cfg.Log.Output = "stdout"
	}
	logger.Init(cfg.Log.Level, cfg.Log.Format, cfg.Log.Output)

	if err := cfg.DB.ValidateAndApplyDefaults(); err != nil {
		return err
	}
	if cfg.Redis.Enable {
		if err := cfg.Redis.ValidateAndApplyDefaults(); err != nil {
			logger.Warn().Err(err).Msg("redis config invalid, fallback to in-memory")
			cfg.Redis.Enable = false
		}
	}
	if err := cfg.JWT.ValidateAndApplyDefaults(); err != nil {
		return err
	}

	if viper.ConfigFileUsed() != "" {
		logger.Info().Str("file", viper.ConfigFileUsed()).Msg("config loaded")
	} else {
		logger.Warn().Msg("no config.yaml found, using env / flags defaults")
	}

	dbPool, err := dbmysql.New(&cfg.DB)
	if err != nil {
		return err
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

	if cfg.DB.EnableAutoMigrate {
		if err := model.Migrate(dbPool.GORM()); err != nil {
			return err
		}
		logger.Info().Msg("auto migrate done")
	}

	initDefaultAdmin(dbPool.GORM())
	initDefaultRoles(dbPool.GORM())
	initDefaultAdminRoleBinding(dbPool.GORM())
	initDefaultPermissions(dbPool.GORM())
	initDefaultAdminRolePermissions(dbPool.GORM())
	initDefaultOAuthClient(dbPool.GORM())

	var rawRedis *redis.Client
	if cfg.Redis.Enable {
		if redisClient, err := dbredis.New(&dbredis.Options{
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

	jwtMgr, err := jwtutil.NewManager(&cfg.JWT, rawRedis)
	if err != nil {
		return err
	}
	logger.Info().
		Int("access_min", cfg.JWT.ExpireMinutes).
		Int("refresh_days", cfg.JWT.RefreshExpireDays).
		Bool("redis", rawRedis != nil).
		Msg("jwt manager ready")

	var sessionStore *session.Store
	if rawRedis != nil {
		sessionStore = session.NewStore(rawRedis)
		logger.Info().Msg("session store ready (redis, ttl=24h)")
	} else {
		logger.Warn().Msg("redis unavailable, session disabled (JWT Bearer only)")
	}

	casbinSvc, err := services.NewCasbinService(dbPool.GORM(), "configs/casbin.conf")
	if err != nil {
		return err
	}
	if err := casbinSvc.EnsureStartupPolicy(context.Background()); err != nil {
		logger.Warn().Err(err).Msg("casbin startup policy load failed, will retry on permission change")
	} else {
		logger.Info().Msg("casbin permission engine ready (policy loaded)")
	}

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
	return nil
}

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

func initDefaultRoles(db *gorm.DB) {
	roleSvc := services.NewRoleService(db)
	if err := roleSvc.EnsureDefaultRoles(context.Background()); err != nil {
		logger.Error().Err(err).Msg("init default roles failed")
		return
	}
	logger.Info().Msg("default system roles ensured (admin/user/viewer)")
}

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

func initDefaultPermissions(db *gorm.DB) {
	permSvc := services.NewPermissionService(db)
	if err := permSvc.EnsureDefaultPermissions(context.Background()); err != nil {
		logger.Error().Err(err).Msg("init default permissions failed")
		return
	}
	logger.Info().Msg("default permissions ensured (24 items)")
}

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