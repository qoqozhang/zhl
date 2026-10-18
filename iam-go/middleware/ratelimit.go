package middleware

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter 限流接口（策略模式）
// - 内存版 MemoryRateLimiter：单实例部署用，零外部依赖
// - RedisRateLimiter：多实例/集群部署用，共享计数
type RateLimiter interface {
	Allow(ctx context.Context, ip string) (allowed bool, remaining int, err error)
}

// -------------------- 内存版 --------------------

type ipRecord struct {
	count     int
	windowEnd time.Time
}

// MemoryRateLimiter 基于内存 map 的固定窗口限流
type MemoryRateLimiter struct {
	mu          sync.Mutex
	records     map[string]*ipRecord
	MaxRequests int
	WindowSize  time.Duration
}

func NewMemoryRateLimiter(maxRequests int, windowSize time.Duration) *MemoryRateLimiter {
	rl := &MemoryRateLimiter{
		records:     make(map[string]*ipRecord),
		MaxRequests: maxRequests,
		WindowSize:  windowSize,
	}
	go rl.cleanup()
	return rl
}

func (rl *MemoryRateLimiter) Allow(_ context.Context, ip string) (bool, int, error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	rec, ok := rl.records[ip]

	if !ok || now.After(rec.windowEnd) {
		rl.records[ip] = &ipRecord{count: 1, windowEnd: now.Add(rl.WindowSize)}
		return true, rl.MaxRequests - 1, nil
	}

	rec.count++
	if rec.count > rl.MaxRequests {
		return false, 0, nil
	}
	return true, rl.MaxRequests - rec.count, nil
}

func (rl *MemoryRateLimiter) cleanup() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, rec := range rl.records {
			if now.After(rec.windowEnd) {
				delete(rl.records, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// -------------------- Redis 版 --------------------

// RedisRateLimiter 基于 Redis 的固定窗口限流
// 用 INCR + EXPIRE 两条命令（原子性由 Redis 保证），比 Lua 更简单，大多数场景足够
// 如需严格滑动窗口 / 毫秒级精度，可换成 ZSET 或 Lua 脚本
type RedisRateLimiter struct {
	client      *redis.Client
	keyPrefix   string        // Redis key 前缀，如 "rl:"，方便区分不同业务限流
	MaxRequests int
	WindowSize  time.Duration
}

// NewRedisRateLimiter 构造 Redis 限流
func NewRedisRateLimiter(client *redis.Client, maxRequests int, windowSize time.Duration, keyPrefix string) *RedisRateLimiter {
	if keyPrefix == "" {
		keyPrefix = "rl:"
	}
	return &RedisRateLimiter{
		client:      client,
		keyPrefix:   keyPrefix,
		MaxRequests: maxRequests,
		WindowSize:  windowSize,
	}
}

// Allow 核心逻辑：
//   1. INCR key → count
//   2. count == 1 ? EXPIRE key windowSize（首次创建，设过期）
//   3. count > MaxRequests ? deny : allow
func (rl *RedisRateLimiter) Allow(ctx context.Context, ip string) (bool, int, error) {
	key := fmt.Sprintf("%s%s", rl.keyPrefix, ip)

	count, err := rl.client.Incr(ctx, key).Result()
	if err != nil {
		return false, 0, fmt.Errorf("redis incr failed: %w", err)
	}

	// 首次 INCR 返回 1，需要设过期时间（窗口到期自动清）
	if count == 1 {
		if err := rl.client.Expire(ctx, key, rl.WindowSize).Err(); err != nil {
			return false, 0, fmt.Errorf("redis expire failed: %w", err)
		}
	}

	if count > int64(rl.MaxRequests) {
		return false, 0, nil
	}
	return true, rl.MaxRequests - int(count), nil
}

// -------------------- 向后兼容 --------------------
// 老代码里用的 NewRateLimiter / *RateLimiter 现在改成下面这个：
//   旧：middleware.NewRateLimiter(n, t)  →  新：middleware.NewMemoryRateLimiter(n, t)