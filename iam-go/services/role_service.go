package services

import (
	"context"
	"errors"
	"fmt"

	"iam-go/model"

	"gorm.io/gorm"
)

var (
	ErrRoleNotFound     = errors.New("role not found")
	ErrRoleExists       = errors.New("role key already exists")
	ErrRoleSystemDelete = errors.New("system built-in role cannot be deleted")
)

type RoleService struct {
	db *gorm.DB
}

func NewRoleService(db *gorm.DB) *RoleService {
	return &RoleService{db: db}
}

type RoleCreateRequest struct {
	RoleKey     string `json:"role_key"     binding:"required,max=64"`
	RoleName    string `json:"role_name"    binding:"required,max=128"`
	Description string `json:"description"  binding:"omitempty,max=255"`
	Status      *int8  `json:"status"       binding:"omitempty,oneof=0 1"`
}

type RoleUpdateRequest struct {
	RoleKey     *string `json:"role_key"     binding:"omitempty,max=64"`
	RoleName    *string `json:"role_name"    binding:"omitempty,max=128"`
	Description *string `json:"description"  binding:"omitempty,max=255"`
	Status      *int8   `json:"status"       binding:"omitempty,oneof=0 1"`
}

type RoleListParams struct {
	Status  *int8  `form:"status"  binding:"omitempty,oneof=0 1"`
	Keyword string `form:"keyword"`
}

type RoleListResult struct {
	Total int64          `json:"total"`
	Items []model.Role   `json:"items"`
}

func (s *RoleService) Create(ctx context.Context, req *RoleCreateRequest) (*model.Role, error) {
	var count int64
	s.db.WithContext(ctx).Model(&model.Role{}).Where("role_key = ?", req.RoleKey).Count(&count)
	if count > 0 {
		return nil, ErrRoleExists
	}

	status := int8(1)
	if req.Status != nil {
		status = *req.Status
	}

	r := &model.Role{
		RoleKey:     req.RoleKey,
		RoleName:    req.RoleName,
		Description: req.Description,
		Status:      status,
		IsSystem:    0,
	}
	if err := s.db.WithContext(ctx).Create(r).Error; err != nil {
		return nil, fmt.Errorf("create role failed: %w", err)
	}
	return r, nil
}

func (s *RoleService) GetByID(ctx context.Context, id uint) (*model.Role, error) {
	var r model.Role
	if err := s.db.WithContext(ctx).First(&r, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("query role failed: %w", err)
	}
	return &r, nil
}

func (s *RoleService) GetByKey(ctx context.Context, roleKey string) (*model.Role, error) {
	var r model.Role
	if err := s.db.WithContext(ctx).Where("role_key = ?", roleKey).First(&r).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("query role failed: %w", err)
	}
	return &r, nil
}

func (s *RoleService) List(ctx context.Context, params *RoleListParams) (*RoleListResult, error) {
	q := s.db.WithContext(ctx).Model(&model.Role{})

	if params.Status != nil {
		q = q.Where("status = ?", *params.Status)
	}
	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		q = q.Where("role_key LIKE ? OR role_name LIKE ?", like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count roles failed: %w", err)
	}

	var items []model.Role
	if err := q.Order("id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list roles failed: %w", err)
	}

	return &RoleListResult{Total: total, Items: items}, nil
}

func (s *RoleService) Update(ctx context.Context, id uint, req *RoleUpdateRequest) (*model.Role, error) {
	r, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}

	if req.RoleKey != nil {
		var count int64
		s.db.WithContext(ctx).Model(&model.Role{}).
			Where("role_key = ? AND id != ?", *req.RoleKey, id).Count(&count)
		if count > 0 {
			return nil, ErrRoleExists
		}
		updates["role_key"] = *req.RoleKey
	}
	if req.RoleName != nil {
		updates["role_name"] = *req.RoleName
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if len(updates) == 0 {
		return r, nil
	}

	if err := s.db.WithContext(ctx).Model(r).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update role failed: %w", err)
	}

	return s.GetByID(ctx, id)
}

func (s *RoleService) Delete(ctx context.Context, id uint) error {
	r, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if r.IsSystem == 1 {
		return ErrRoleSystemDelete
	}

	if err := s.db.WithContext(ctx).Delete(r).Error; err != nil {
		return fmt.Errorf("delete role failed: %w", err)
	}
	return nil
}

// EnsureDefaultRoles 启动时确保系统内置角色存在
// 幂等：重复调用不会产生副作用
func (s *RoleService) EnsureDefaultRoles(ctx context.Context) error {
	defaults := []*model.Role{
		{
			RoleKey:     "admin",
			RoleName:    "系统管理员",
			Description: "拥有系统所有权限，不可删除",
			Status:      1,
			IsSystem:    1,
		},
		{
			RoleKey:     "user",
			RoleName:    "普通用户",
			Description: "基础操作权限",
			Status:      1,
			IsSystem:    1,
		},
		{
			RoleKey:     "viewer",
			RoleName:    "只读用户",
			Description: "仅查看权限，不可修改数据",
			Status:      1,
			IsSystem:    1,
		},
	}

	for _, dr := range defaults {
		var count int64
		s.db.WithContext(ctx).Model(&model.Role{}).Where("role_key = ?", dr.RoleKey).Count(&count)
		if count > 0 {
			continue
		}
		if err := s.db.WithContext(ctx).Create(dr).Error; err != nil {
			return fmt.Errorf("create default role %s failed: %w", dr.RoleKey, err)
		}
	}
	return nil
}