package middleware

import (
	"net/http"
	"time"

	"iam-go/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/phuslu/log"
)

// Logger 结构化请求日志中间件
// 输出字段: method / path / status / latency / ip / ua / errors
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method
		ip := c.ClientIP()
		ua := c.Request.UserAgent()
		query := c.Request.URL.RawQuery

		c.Next()

		cost := time.Since(start)
		status := c.Writer.Status()

		e := logger.Info().
			Str("method", method).
			Str("path", path).
			Int("status", status).
			Str("latency", cost.String()).
			Str("ip", ip).
			Str("ua", ua)

		if query != "" {
			e.Str("query", query)
		}
		if len(c.Errors) > 0 {
			e.Str("errors", c.Errors.String())
		}

		switch {
		case status >= 500:
			e.Level = log.ErrorLevel
			e.Msg("server error")
		case status >= 400:
			e.Level = log.WarnLevel
			e.Msg("client error")
		default:
			e.Msg("request complete")
		}
	}
}

// CORSMiddleware 跨域中间件
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// RateLimitMiddleware 把 RateLimiter（接口）包装成 gin 中间件
// 同时兼容内存版 MemoryRateLimiter 和 Redis 版 RedisRateLimiter
func RateLimitMiddleware(rl RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		allowed, remaining, err := rl.Allow(c.Request.Context(), ip)
		if err != nil {
			logger.Warn().Str("ip", ip).Err(err).Msg("rate limiter error, fail-open")
			c.Header("X-RateLimit-Error", err.Error())
			c.Next()
			return
		}

		switch v := rl.(type) {
		case *MemoryRateLimiter:
			c.Header("X-RateLimit-Limit", intToString(v.MaxRequests))
		case *RedisRateLimiter:
			c.Header("X-RateLimit-Limit", intToString(v.MaxRequests))
		}
		c.Header("X-RateLimit-Remaining", intToString(remaining))

		if !allowed {
			logger.Warn().Str("ip", ip).Str("path", c.Request.URL.Path).Msg("rate limit exceeded")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    42900,
				"message": "too many requests, please try again later",
			})
			return
		}
		c.Next()
	}
}

func intToString(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}