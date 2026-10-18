package redis

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Options Redis 连接配置
type Options struct {
	Addr         string        // host:port，如 127.0.0.1:6379
	Password     string        // 无密码留空
	DB           int           // 默认 0
	DialTimeout  time.Duration // 连接建立超时，默认 5s
	ReadTimeout  time.Duration // 读超时，默认 3s
	WriteTimeout time.Duration // 写超时，默认 3s
	PoolSize     int           // 连接池大小，默认 10 * CPU 核数
	MinIdleConns int           // 最小空闲连接数，默认 0
}

// Client Redis 客户端封装
type Client struct {
	mu  sync.Mutex
	rdb *redis.Client
}

var (
	defaultInstance *Client
	defaultOnce     sync.Once
)

// New 创建独立的 Redis 客户端
func New(opts *Options) (*Client, error) {
	if opts == nil {
		return nil, fmt.Errorf("redis options is nil")
	}
	if opts.Addr == "" {
		return nil, fmt.Errorf("redis addr is required")
	}

	if opts.DialTimeout == 0 {
		opts.DialTimeout = 5 * time.Second
	}
	if opts.ReadTimeout == 0 {
		opts.ReadTimeout = 3 * time.Second
	}
	if opts.WriteTimeout == 0 {
		opts.WriteTimeout = 3 * time.Second
	}
	if opts.PoolSize == 0 {
		opts.PoolSize = 16
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:         opts.Addr,
		Password:     opts.Password,
		DB:           opts.DB,
		DialTimeout:  opts.DialTimeout,
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
		PoolSize:     opts.PoolSize,
		MinIdleConns: opts.MinIdleConns,
	})

	ctx, cancel := context.WithTimeout(context.Background(), opts.DialTimeout)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

// Default 获取全局单例（懒加载）
func Default(opts *Options) (*Client, error) {
	var onceErr error
	defaultOnce.Do(func() {
		defaultInstance, onceErr = New(opts)
	})
	return defaultInstance, onceErr
}

// Raw 拿到底层 *redis.Client，直接用完整 API
func (c *Client) Raw() *redis.Client {
	return c.rdb
}

// Close 关闭客户端（通常不需要调，进程退出 Go runtime 会清理）
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rdb.Close()
}