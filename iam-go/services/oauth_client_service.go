package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"iam-go/model"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrOAuthClientNotFound   = errors.New("oauth client not found")
	ErrOAuthClientExists     = errors.New("oauth client_id already exists")
	ErrOAuthClientDisabled   = errors.New("oauth client is disabled")
	ErrOAuthRedirectMismatch = errors.New("redirect_uri not registered")
	ErrOAuthClientSecret     = errors.New("invalid client_secret")
)

type OAuthClientService struct {
	db *gorm.DB
}

func NewOAuthClientService(db *gorm.DB) *OAuthClientService {
	return &OAuthClientService{db: db}
}

func generateRandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

type OAuthClientCreateRequest struct {
	ClientID     string   `json:"client_id"     binding:"required,max=128"`
	ClientName   string   `json:"client_name"   binding:"required,max=128"`
	RedirectURIs []string `json:"redirect_uris" binding:"required,min=1"`
	Scopes       string   `json:"scopes"        binding:"omitempty,max=512"`
}

type OAuthClientCreateResponse struct {
	ID              uint   `json:"id"`
	ClientID        string `json:"client_id"`
	ClientSecret    string `json:"client_secret"`
	ClientName      string `json:"client_name"`
	RedirectURIs    string `json:"redirect_uris"`
	Scopes          string `json:"scopes"`
	Status          int8   `json:"status"`
}

func (s *OAuthClientService) Create(ctx context.Context, req *OAuthClientCreateRequest) (*OAuthClientCreateResponse, error) {
	var count int64
	s.db.WithContext(ctx).Model(&model.OAuthClient{}).
		Where("client_id = ?", req.ClientID).Count(&count)
	if count > 0 {
		return nil, ErrOAuthClientExists
	}

	secret, err := generateRandomHex(32)
	if err != nil {
		return nil, fmt.Errorf("generate client_secret failed: %w", err)
	}
	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash client_secret failed: %w", err)
	}

	client := &model.OAuthClient{
		ClientID:         req.ClientID,
		ClientSecretHash: string(hashedSecret),
		ClientName:       req.ClientName,
		RedirectURIs:     strings.Join(req.RedirectURIs, ","),
		Scopes:           req.Scopes,
		Status:           1,
	}
	if err := s.db.WithContext(ctx).Create(client).Error; err != nil {
		return nil, fmt.Errorf("create oauth client failed: %w", err)
	}

	return &OAuthClientCreateResponse{
		ID:           client.ID,
		ClientID:     client.ClientID,
		ClientSecret: secret,
		ClientName:   client.ClientName,
		RedirectURIs: client.RedirectURIs,
		Scopes:       client.Scopes,
		Status:       client.Status,
	}, nil
}

// GetByID 通过 client_id 查客户端并校验状态（OAuth 鉴权流程用）
func (s *OAuthClientService) GetByID(ctx context.Context, clientID string) (*model.OAuthClient, error) {
	c, err := s.GetByIDRaw(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if c.Status != 1 {
		return c, ErrOAuthClientDisabled
	}
	return c, nil
}

// GetByIDRaw 按 client_id 查，不校验 status（admin 后台用）
func (s *OAuthClientService) GetByIDRaw(ctx context.Context, clientID string) (*model.OAuthClient, error) {
	var c model.OAuthClient
	if err := s.db.WithContext(ctx).Where("client_id = ?", clientID).First(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOAuthClientNotFound
		}
		return nil, fmt.Errorf("query oauth client failed: %w", err)
	}
	return &c, nil
}

// GetByIDByNum 按数字主键查（admin 后台用）
func (s *OAuthClientService) GetByIDByNum(ctx context.Context, id uint) (*model.OAuthClient, error) {
	var c model.OAuthClient
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOAuthClientNotFound
		}
		return nil, fmt.Errorf("query oauth client failed: %w", err)
	}
	return &c, nil
}

// Authenticate 用 client_id + client_secret 认证
func (s *OAuthClientService) Authenticate(ctx context.Context, clientID, clientSecret string) (*model.OAuthClient, error) {
	c, err := s.GetByID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(c.ClientSecretHash), []byte(clientSecret)); err != nil {
		return nil, ErrOAuthClientSecret
	}
	return c, nil
}

// ValidateRedirectURI 校验 redirect_uri 是否在客户端注册列表里
func (s *OAuthClientService) ValidateRedirectURI(c *model.OAuthClient, redirectURI string) error {
	if !c.ValidateRedirectURI(redirectURI) {
		return ErrOAuthRedirectMismatch
	}
	return nil
}

// ResetSecret 重置 client_secret（返回新明文密钥）
func (s *OAuthClientService) ResetSecret(ctx context.Context, clientID string) (string, error) {
	c, err := s.GetByID(ctx, clientID)
	if err != nil {
		return "", err
	}
	secret, err := generateRandomHex(32)
	if err != nil {
		return "", fmt.Errorf("generate client_secret failed: %w", err)
	}
	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash client_secret failed: %w", err)
	}
	if err := s.db.WithContext(ctx).Model(c).Update("client_secret_hash", string(hashedSecret)).Error; err != nil {
		return "", fmt.Errorf("reset secret failed: %w", err)
	}
	return secret, nil
}

// EnsureDefaultClient 启动时确保存在默认客户端
func (s *OAuthClientService) EnsureDefaultClient(ctx context.Context) error {
	var count int64
	s.db.WithContext(ctx).Model(&model.OAuthClient{}).Where("client_id = ?", "default_web").Count(&count)
	if count > 0 {
		return nil
	}
	_, err := s.Create(ctx, &OAuthClientCreateRequest{
		ClientID:     "default_web",
		ClientName:   "默认Web应用",
		RedirectURIs: []string{"http://localhost:3000/callback", "http://127.0.0.1:3000/callback"},
		Scopes:       "openid profile email user:read role:read",
	})
	return err
}

type OAuthClientListParams struct {
	Status  *int8  `form:"status"  binding:"omitempty,oneof=0 1"`
	Keyword string `form:"keyword"`
}

type OAuthClientListResult struct {
	Total int64                      `json:"total"`
	Items []*model.OAuthClient       `json:"items"`
}

func (s *OAuthClientService) List(ctx context.Context, params *OAuthClientListParams) (*OAuthClientListResult, error) {
	q := s.db.WithContext(ctx).Model(&model.OAuthClient{})

	if params.Status != nil {
		q = q.Where("status = ?", *params.Status)
	}
	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		q = q.Where("client_id LIKE ? OR client_name LIKE ?", like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count oauth clients failed: %w", err)
	}

	var items []*model.OAuthClient
	if err := q.Order("id DESC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list oauth clients failed: %w", err)
	}

	return &OAuthClientListResult{Total: total, Items: items}, nil
}

type OAuthClientUpdateRequest struct {
	ClientName   *string  `json:"client_name,omitempty"   binding:"omitempty,max=128"`
	RedirectURIs []string `json:"redirect_uris,omitempty" binding:"omitempty,min=1"`
	Scopes       *string  `json:"scopes,omitempty"        binding:"omitempty,max=512"`
	Status       *int8    `json:"status,omitempty"       binding:"omitempty,oneof=0 1"`
}

func (s *OAuthClientService) Update(ctx context.Context, id uint, req *OAuthClientUpdateRequest) (*model.OAuthClient, error) {
	c, err := s.GetByIDByNum(ctx, id)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.ClientName != nil {
		updates["client_name"] = *req.ClientName
	}
	if req.RedirectURIs != nil {
		updates["redirect_uris"] = strings.Join(req.RedirectURIs, ",")
	}
	if req.Scopes != nil {
		updates["scopes"] = *req.Scopes
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if len(updates) > 0 {
		if err := s.db.WithContext(ctx).Model(c).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("update oauth client failed: %w", err)
		}
		if err := s.db.WithContext(ctx).First(c, id).Error; err != nil {
			return nil, fmt.Errorf("reload oauth client failed: %w", err)
		}
	}

	return c, nil
}

func (s *OAuthClientService) Delete(ctx context.Context, id uint) error {
	c, err := s.GetByIDByNum(ctx, id)
	if err != nil {
		return err
	}

	var codeCount int64
	s.db.WithContext(ctx).Model(&model.OAuthAuthorizationCode{}).
		Where("client_id = ?", c.ClientID).Count(&codeCount)
	if codeCount > 0 {
		return fmt.Errorf("该客户端存在 %d 条授权码记录，请先清理", codeCount)
	}

	if err := s.db.WithContext(ctx).Delete(c).Error; err != nil {
		return fmt.Errorf("delete oauth client failed: %w", err)
	}
	return nil
}