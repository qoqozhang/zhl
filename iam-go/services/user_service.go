package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"iam-go/model"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidPassword    = errors.New("password too short, min 6 chars")
	ErrInvalidPageParam   = errors.New("page/page_size must be positive")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserDisabled       = errors.New("user account is disabled")
)

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

type CreateRequest struct {
	Username     string   `json:"username"       binding:"required,min=3,max=64"`
	Password     string   `json:"password"       binding:"required,min=6,max=64"`
	RealName     string   `json:"real_name"      binding:"required,max=64"`
	Email        string   `json:"email"          binding:"omitempty,email,max=128"`
	Phone        string   `json:"phone"          binding:"omitempty,max=32"`
	AvatarURL    string   `json:"avatar_url"     binding:"omitempty,max=255"`
	DepartmentID *uint64  `json:"department_id"  binding:"omitempty"`
	RoleKeys     []string `json:"role_keys"      binding:"omitempty"`
}

type UpdateRequest struct {
	Username     *string  `json:"username"       binding:"omitempty,min=3,max=64"`
	Email        *string  `json:"email"          binding:"omitempty,email,max=128"`
	Phone        *string  `json:"phone"          binding:"omitempty,max=32"`
	RealName     *string  `json:"real_name"      binding:"omitempty,max=64"`
	AvatarURL    *string  `json:"avatar_url"     binding:"omitempty,max=255"`
	DepartmentID *uint64  `json:"department_id"  binding:"omitempty"`
	Status       *int8    `json:"status"         binding:"omitempty,oneof=0 1"`
	Password     *string  `json:"password"       binding:"omitempty,min=6,max=64"`
	RoleKeys     []string `json:"role_keys"      binding:"omitempty,unique"`
}

type ListParams struct {
	Page         int     `form:"page"          binding:"omitempty,min=1"`
	PageSize     int     `form:"page_size"     binding:"omitempty,min=1,max=100"`
	Keyword      string  `form:"keyword"`
	Status       *int8   `form:"status"        binding:"omitempty,oneof=0 1"`
	DepartmentID *uint64 `form:"department_id" binding:"omitempty"`
	RoleKey      *string `form:"role_key"`
}

type ListResult struct {
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
	Items    []model.User `json:"items"`
}

func (s *UserService) Create(ctx context.Context, req *CreateRequest) (*model.User, error) {
	var count int64
	s.db.WithContext(ctx).Model(&model.User{}).
		Where("username = ? OR email = ? OR phone = ?", req.Username, req.Email, req.Phone).
		Count(&count)
	if count > 0 {
		return nil, ErrUserExists
	}

	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password failed: %w", err)
	}

	u := &model.User{
		Username:     req.Username,
		PasswordHash: string(hashedPwd),
		RealName:     req.RealName,
		Email:        req.Email,
		Phone:        req.Phone,
		AvatarURL:    req.AvatarURL,
		DepartmentID: req.DepartmentID,
		Status:       1,
	}

	roleKeys := req.RoleKeys
	if len(roleKeys) == 0 {
		roleKeys = []string{"user"}
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(u).Error; err != nil {
			return fmt.Errorf("create user failed: %w", err)
		}
		urSvc := NewUserRoleService(tx)
		if err := urSvc.ReplaceRoles(ctx, u.ID, roleKeys); err != nil {
			return fmt.Errorf("bind roles failed: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *UserService) GetByID(ctx context.Context, id uint) (*model.User, error) {
	var u model.User
	if err := s.db.WithContext(ctx).First(&u, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("query user failed: %w", err)
	}
	return &u, nil
}

func (s *UserService) GetWithRoleKeys(ctx context.Context, id uint) (*model.User, []string, error) {
	u, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	urSvc := NewUserRoleService(s.db)
	keys, err := urSvc.GetRoleKeysForUser(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return u, keys, nil
}

func (s *UserService) List(ctx context.Context, params *ListParams) (*ListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 10
	}

	q := s.db.WithContext(ctx).Model(&model.User{})

	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		q = q.Where("username LIKE ? OR real_name LIKE ? OR email LIKE ?", like, like, like)
	}
	if params.Status != nil {
		q = q.Where("status = ?", *params.Status)
	}
	if params.DepartmentID != nil {
		q = q.Where("department_id = ?", *params.DepartmentID)
	}
	if params.RoleKey != nil && *params.RoleKey != "" {
		q = q.Joins("JOIN sys_user_role ur ON ur.user_id = sys_user.id").
			Joins("JOIN sys_role r ON r.id = ur.role_id AND r.role_key = ?", *params.RoleKey)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count users failed: %w", err)
	}

	var items []model.User
	offset := (params.Page - 1) * params.PageSize
	if err := q.Order("id DESC").Offset(offset).Limit(params.PageSize).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list users failed: %w", err)
	}

	return &ListResult{
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
		Items:    items,
	}, nil
}

func (s *UserService) Update(ctx context.Context, id uint, req *UpdateRequest) (*model.User, error) {
	u, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}

	if req.Username != nil {
		updates["username"] = *req.Username
	}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.Phone != nil {
		updates["phone"] = *req.Phone
	}
	if req.RealName != nil {
		updates["real_name"] = *req.RealName
	}
	if req.AvatarURL != nil {
		updates["avatar_url"] = *req.AvatarURL
	}
	if req.DepartmentID != nil {
		updates["department_id"] = *req.DepartmentID
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Password != nil {
		hashed, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hash password failed: %w", err)
		}
		updates["password_hash"] = string(hashed)
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(updates) > 0 {
			if err := tx.Model(u).Updates(updates).Error; err != nil {
				return fmt.Errorf("update user failed: %w", err)
			}
		}
		if req.RoleKeys != nil {
			urSvc := NewUserRoleService(tx)
			if err := urSvc.ReplaceRoles(ctx, id, req.RoleKeys); err != nil {
				return fmt.Errorf("replace roles failed: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.GetByID(ctx, id)
}

func (s *UserService) Delete(ctx context.Context, id uint) error {
	u, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", id).Delete(&model.UserRole{}).Error; err != nil {
			return fmt.Errorf("clear user roles failed: %w", err)
		}
		if err := tx.Delete(u).Error; err != nil {
			return fmt.Errorf("delete user failed: %w", err)
		}
		return nil
	})
	return err
}

type LoginRequest struct {
	Username string `json:"username"  binding:"required"`
	Password string `json:"password"  binding:"required,min=6"`
	ClientIP string `json:"-"`
}

func (s *UserService) Login(ctx context.Context, req *LoginRequest) (*model.User, []string, error) {
	var u model.User
	if err := s.db.WithContext(ctx).
		Where("username = ?", req.Username).
		First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrInvalidCredentials
		}
		return nil, nil, fmt.Errorf("query user failed: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	if u.Status != 1 {
		return nil, nil, ErrUserDisabled
	}

	now := time.Now()
	updates := map[string]interface{}{"last_login_at": now}
	if req.ClientIP != "" {
		updates["last_login_ip"] = req.ClientIP
	}
	s.db.WithContext(ctx).Model(&u).Updates(updates)

	urSvc := NewUserRoleService(s.db)
	roleKeys, err := urSvc.GetRoleKeysForUser(ctx, u.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("query user roles failed: %w", err)
	}

	return &u, roleKeys, nil
}

func (s *UserService) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("query user failed: %w", err)
	}
	return &u, nil
}