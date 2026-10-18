package configs

import (
	"fmt"
	"net/url"
)

// Config 全局配置
// 后续可替换为 viper 从 yaml/env 加载
type Config struct {
	Port  string      `mapstructure:"port" json:"port" yaml:"port"`
	DB    DBConfig    `mapstructure:"db" json:"db" yaml:"db"`
	Redis RedisConfig `mapstructure:"redis" json:"redis" yaml:"redis"`
	JWT   JWTConfig   `mapstructure:"jwt" json:"jwt" yaml:"jwt"`
}

// DBConfig MySQL 连接池配置（配合 GORM 使用）
type DBConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`

	MaxOpenConns    int `yaml:"max_open_conns"`
	MaxIdleConns    int `yaml:"max_idle_conns"`
	ConnMaxLifetime int `yaml:"conn_max_lifetime"`  // 秒
	ConnMaxIdleTime int `yaml:"conn_max_idle_time"` // 秒

	Charset   string `yaml:"charset"`
	ParseTime bool   `yaml:"parse_time"`
	SSLMode   string `yaml:"ssl_mode"`
	Timeout   int    `yaml:"timeout"`

	LogLevel          int  `yaml:"log_level"`           // GORM 日志级别: 0=Silent 1=Error 2=Warn 3=Info
	EnableAutoMigrate bool `yaml:"enable_auto_migrate"` // 启动时自动建表（dev 开，prod 关）
}

// ValidateAndApplyDefaults 校验并填充默认值
// 必须在 configs 包里定义（Go 不允许给非本地类型加方法）
func (c *DBConfig) ValidateAndApplyDefaults() error {
	if c.Host == "" {
		return fmt.Errorf("db.host is required")
	}
	if c.User == "" {
		return fmt.Errorf("db.user is required")
	}
	if c.Database == "" {
		return fmt.Errorf("db.database is required")
	}

	if c.Port == 0 {
		c.Port = 3306
	}
	if c.MaxOpenConns == 0 {
		c.MaxOpenConns = 100
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = 10
	}
	if c.ConnMaxLifetime == 0 {
		c.ConnMaxLifetime = 300
	}
	if c.ConnMaxIdleTime == 0 {
		c.ConnMaxIdleTime = 60
	}
	if c.Charset == "" {
		c.Charset = "utf8mb4"
	}
	if !c.ParseTime {
		c.ParseTime = true
	}
	if c.SSLMode == "" {
		c.SSLMode = "false"
	}
	if c.Timeout == 0 {
		c.Timeout = 5
	}
	if c.LogLevel == 0 {
		c.LogLevel = 2
	}

	if c.MaxIdleConns > c.MaxOpenConns {
		c.MaxIdleConns = c.MaxOpenConns
	}
	return nil
}

// BuildDSN 构造 MySQL DSN（密码做 URL 编码，避免特殊字符导致解析错误）
func (c *DBConfig) BuildDSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=Local&timeout=%ds&sslmode=%s",
		c.User,
		url.QueryEscape(c.Password),
		c.Host,
		c.Port,
		c.Database,
		c.Charset,
		c.ParseTime,
		c.Timeout,
		c.SSLMode,
	)
}

// RedisConfig Redis 连接配置
type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`

	// 连接池 & 超时
	DialTimeout  int `yaml:"dial_timeout"`  // 秒
	ReadTimeout  int `yaml:"read_timeout"`  // 秒
	WriteTimeout int `yaml:"write_timeout"` // 秒
	PoolSize     int `yaml:"pool_size"`
	MinIdleConns int `yaml:"min_idle_conns"`

	// 业务开关：是否启用 Redis（关闭则走内存限流）
	Enable bool `yaml:"enable"`
}

// ValidateAndApplyDefaults 校验 + 填默认值
func (c *RedisConfig) ValidateAndApplyDefaults() error {
	if c.Host == "" {
		return fmt.Errorf("redis.host is required")
	}
	if c.Port == 0 {
		c.Port = 6379
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = 5
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 3
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 3
	}
	if c.PoolSize == 0 {
		c.PoolSize = 16
	}
	return nil
}

// Addr 组装 host:port
func (c *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret           string `yaml:"secret"`            // 签名密钥（生产必须配置）
	ExpireMinutes    int    `yaml:"expire_minutes"`    // Access Token 过期时间（分钟）
	RefreshExpireDays int   `yaml:"refresh_expire_days"` // Refresh Token 过期时间（天）
	Issuer           string `yaml:"issuer"`            // 签发者标识（可选）
	RefreshKeyPrefix string `yaml:"refresh_key_prefix"` // Redis key 前缀，默认 "refresh:"
}

// ValidateAndApplyDefaults 校验 + 填默认值
func (c *JWTConfig) ValidateAndApplyDefaults() error {
	if c.Secret == "" {
		return fmt.Errorf("jwt.secret is required")
	}
	if c.ExpireMinutes == 0 {
		c.ExpireMinutes = 30 // Access Token 默认 30 分钟
	}
	if c.RefreshExpireDays == 0 {
		c.RefreshExpireDays = 7 // Refresh Token 默认 7 天
	}
	if c.Issuer == "" {
		c.Issuer = "iam-go"
	}
	if c.RefreshKeyPrefix == "" {
		c.RefreshKeyPrefix = "refresh:"
	}
	return nil
}