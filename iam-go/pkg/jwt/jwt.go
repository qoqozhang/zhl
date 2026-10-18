package jwtutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"iam-go/configs"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// AccessClaims Access Token payload
type AccessClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	ClientID string `json:"client_id,omitempty"`
	Scope    string `json:"scope,omitempty"`
	jwt.RegisteredClaims
}

// RefreshClaims Refresh Token payload
type RefreshClaims struct {
	jwt.RegisteredClaims
	UserID uint `json:"user_id"`
}

// Manager JWT 管理器
type Manager struct {
	secret           []byte
	accessExpire     time.Duration
	refreshExpire    time.Duration
	refreshKeyPrefix string
	issuer           string
	redis            *redis.Client // 可选，传 nil 则 Refresh Token 功能禁用
}

// NewManager 构造器
// redisClient 可以传 nil（仅 Access Token 功能可用）
func NewManager(cfg *configs.JWTConfig, redisClient *redis.Client) (*Manager, error) {
	if err := cfg.ValidateAndApplyDefaults(); err != nil {
		return nil, err
	}
	return &Manager{
		secret:           []byte(cfg.Secret),
		accessExpire:     time.Duration(cfg.ExpireMinutes) * time.Minute,
		refreshExpire:    time.Duration(cfg.RefreshExpireDays) * 24 * time.Hour,
		refreshKeyPrefix: cfg.RefreshKeyPrefix,
		issuer:           cfg.Issuer,
		redis:            redisClient,
	}, nil
}

// ---------------- Access Token ----------------

// GenerateAccess 签发 Access Token（无状态，不存 Redis）
func (m *Manager) GenerateAccess(userID uint, username, role string) (string, error) {
	return m.GenerateAccessWithOAuth(userID, username, role, "", "")
}

// GenerateAccessWithOAuth OAuth 场景签发 Access Token（带 client_id + scope）
func (m *Manager) GenerateAccessWithOAuth(userID uint, username, role, clientID, scope string) (string, error) {
	now := time.Now()
	claims := AccessClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		ClientID: clientID,
		Scope:    scope,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessExpire)),
			Subject:   fmt.Sprintf("%d", userID),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// ParseAccess 校验 Access Token（只验签名 + 过期，无 Redis 查）
func (m *Manager) ParseAccess(tokenStr string) (*AccessClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &AccessClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*AccessClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid access token")
	}
	return claims, nil
}

// ---------------- Refresh Token ----------------

// ErrRefreshDisabled Refresh Token 功能未启用（Manager 没注入 Redis）
var ErrRefreshDisabled = errors.New("refresh token disabled: no redis client")

// GenerateRefresh 签发 Refresh Token
// 额外做了：jti 写 Redis（SETNX + 过期时间 = refreshExpire）
func (m *Manager) GenerateRefresh(ctx context.Context, userID uint) (tokenStr string, jti string, err error) {
	if m.redis == nil {
		return "", "", ErrRefreshDisabled
	}

	jti = newJTI()
	now := time.Now()

	claims := RefreshClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshExpire)),
			ID:        jti,
			Subject:   fmt.Sprintf("%d", userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err = token.SignedString(m.secret)
	if err != nil {
		return "", "", err
	}

	// jti 存 Redis，SET + 过期
	key := m.refreshKeyPrefix + jti
	if err := m.redis.Set(ctx, key, userID, m.refreshExpire).Err(); err != nil {
		return "", "", fmt.Errorf("store refresh token in redis failed: %w", err)
	}

	return tokenStr, jti, nil
}

// ParseRefresh 校验 Refresh Token
// 先验签，再查 Redis 有没有这个 jti（没有=已撤销）
func (m *Manager) ParseRefresh(ctx context.Context, tokenStr string) (*RefreshClaims, error) {
	if m.redis == nil {
		return nil, ErrRefreshDisabled
	}

	token, err := jwt.ParseWithClaims(tokenStr, &RefreshClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*RefreshClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid refresh token")
	}

	// 查 Redis 里有没有这个 jti
	if claims.ID == "" {
		return nil, errors.New("refresh token missing jti")
	}
	key := m.refreshKeyPrefix + claims.ID
	exists, err := m.redis.Exists(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis check failed: %w", err)
	}
	if exists == 0 {
		return nil, errors.New("refresh token revoked or expired")
	}

	return claims, nil
}

// RevokeRefresh 撤销 Refresh Token（登出）
func (m *Manager) RevokeRefresh(ctx context.Context, jti string) error {
	if m.redis == nil {
		return ErrRefreshDisabled
	}
	key := m.refreshKeyPrefix + jti
	return m.redis.Del(ctx, key).Err()
}

// newJTI 生成唯一 Token ID（32 字符 hex）
func newJTI() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 极端情况：rand 挂了，用时间戳兜底
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}