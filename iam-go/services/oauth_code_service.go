package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"iam-go/model"

	"gorm.io/gorm"
)

var (
	ErrOAuthCodeNotFound  = errors.New("authorization code not found")
	ErrOAuthCodeUsed      = errors.New("authorization code already used")
	ErrOAuthCodeExpired   = errors.New("authorization code expired")
	ErrOAuthCodeMismatch  = errors.New("authorization code mismatch")
)

type OAuthCodeService struct {
	db *gorm.DB
}

func NewOAuthCodeService(db *gorm.DB) *OAuthCodeService {
	return &OAuthCodeService{db: db}
}

func generateCode(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

const codeExpireDuration = 5 * time.Minute

// Create 生成授权码并存储（存 hash，返回明文 code）
func (s *OAuthCodeService) Create(ctx context.Context, clientID string, userID uint, redirectURI string, scopes string) (string, error) {
	code, err := generateCode(32)
	if err != nil {
		return "", fmt.Errorf("generate code failed: %w", err)
	}

	c := &model.OAuthAuthorizationCode{
		CodeHash:    hashCode(code),
		ClientID:    clientID,
		UserID:      userID,
		RedirectURI: redirectURI,
		Scopes:      scopes,
		ExpiresAt:   time.Now().Add(codeExpireDuration),
		Used:        0,
	}
	if err := s.db.WithContext(ctx).Create(c).Error; err != nil {
		return "", fmt.Errorf("store authorization code failed: %w", err)
	}
	return code, nil
}

// Consume 校验并使用授权码（一次性）
func (s *OAuthCodeService) Consume(ctx context.Context, code, clientID, redirectURI string) (*model.OAuthAuthorizationCode, error) {
	codeHash := hashCode(code)

	var c model.OAuthAuthorizationCode
	if err := s.db.WithContext(ctx).Where("code_hash = ?", codeHash).First(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOAuthCodeNotFound
		}
		return nil, fmt.Errorf("query authorization code failed: %w", err)
	}

	if c.ClientID != clientID {
		return nil, ErrOAuthCodeMismatch
	}
	if c.RedirectURI != redirectURI {
		return nil, ErrOAuthCodeMismatch
	}
	if c.Used == 1 {
		return nil, ErrOAuthCodeUsed
	}
	if time.Now().After(c.ExpiresAt) {
		return nil, ErrOAuthCodeExpired
	}

	if err := s.db.WithContext(ctx).Model(&c).Update("used", 1).Error; err != nil {
		return nil, fmt.Errorf("mark code used failed: %w", err)
	}

	return &c, nil
}

// VerifyPKCE 校验 PKCE code_verifier（S256 或 plain）
func VerifyPKCE(codeVerifier, codeChallenge, codeChallengeMethod string) bool {
	if codeVerifier == "" || codeChallenge == "" {
		return true
	}
	switch codeChallengeMethod {
	case "S256":
		sum := sha256.Sum256([]byte(codeVerifier))
		computed := base64.RawURLEncoding.EncodeToString(sum[:])
		return computed == codeChallenge
	case "plain":
		return codeVerifier == codeChallenge
	case "":
		return true
	default:
		return false
	}
}

// CleanupExpired 清理过期未使用的授权码（可定时调用）
func (s *OAuthCodeService) CleanupExpired(ctx context.Context) (int64, error) {
	res := s.db.WithContext(ctx).
		Where("expires_at < ? OR used = 1", time.Now().Add(-1*time.Hour)).
		Delete(&model.OAuthAuthorizationCode{})
	return res.RowsAffected, res.Error
}