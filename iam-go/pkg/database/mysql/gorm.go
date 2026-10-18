package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"iam-go/configs"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB GORM 连接池封装
// 底层是 *gorm.DB，通过 db.DB() 能拿到原生 *sql.DB 调标准库
type DB struct {
	mu sync.Mutex
	db *gorm.DB
}

var (
	defaultInstance *DB
	defaultOnce     sync.Once
)

// New 根据配置创建独立的 GORM 连接
func New(cfg *configs.DBConfig) (*DB, error) {
	if cfg == nil {
		return nil, fmt.Errorf("db config is nil")
	}
	if err := cfg.ValidateAndApplyDefaults(); err != nil {
		return nil, err
	}

	dsn := cfg.BuildDSN()

	gormLogger := logger.Default.LogMode(logger.LogLevel(cfg.LogLevel))

	gormDB, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger:               gormLogger,
		DisableAutomaticPing: false,
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open failed: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB failed: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	sqlDB.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTime) * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("mysql ping failed: %w", err)
	}

	return &DB{db: gormDB}, nil
}

// Default 获取全局单例（懒加载）
func Default(cfg *configs.DBConfig) (*DB, error) {
	var onceErr error
	defaultOnce.Do(func() {
		defaultInstance, onceErr = New(cfg)
	})
	return defaultInstance, onceErr
}

// GORM 拿到底层 *gorm.DB
func (p *DB) GORM() *gorm.DB {
	return p.db
}

// SQL 拿到底层 *sql.DB（标准库完整能力）
func (p *DB) SQL() (*sql.DB, error) {
	return p.db.DB()
}

// Close 关闭连接池
func (p *DB) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	sqlDB, err := p.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Stats 返回连接池状态
func (p *DB) Stats() (sql.DBStats, error) {
	sqlDB, err := p.db.DB()
	if err != nil {
		return sql.DBStats{}, err
	}
	return sqlDB.Stats(), nil
}