package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"iam-go/model"

	"gorm.io/gorm"
)

type UserRoleService struct {
	db *gorm.DB
}

func NewUserRoleService(db *gorm.DB) *UserRoleService {
	return &UserRoleService{db: db}
}

// Bind 绑定一个角色给用户（幂等，已存在则跳过）
func (s *UserRoleService) Bind(ctx context.Context, userID uint, roleID uint) error {
	var count int64
	s.db.WithContext(ctx).Model(&model.UserRole{}).
		Where("user_id = ? AND role_id = ?", userID, roleID).Count(&count)
	if count > 0 {
		return nil
	}
	ur := &model.UserRole{UserID: userID, RoleID: roleID, CreatedAt: time.Now()}
	if err := s.db.WithContext(ctx).Create(ur).Error; err != nil {
		return fmt.Errorf("bind user-role failed: %w", err)
	}
	return nil
}

// Unbind 解绑一个角色
func (s *UserRoleService) Unbind(ctx context.Context, userID uint, roleID uint) error {
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND role_id = ?", userID, roleID).
		Delete(&model.UserRole{}).Error; err != nil {
		return fmt.Errorf("unbind user-role failed: %w", err)
	}
	return nil
}

// ReplaceRoles 用一组 role_keys 完全替换用户的角色绑定（事务内：删旧 → 插新）
// role_keys 为空数组则清空所有角色
func (s *UserRoleService) ReplaceRoles(ctx context.Context, userID uint, roleKeys []string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
			return fmt.Errorf("clear old roles failed: %w", err)
		}
		for _, key := range roleKeys {
			var role model.Role
			if err := tx.WithContext(ctx).Where("role_key = ?", key).First(&role).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("role not found: %s", key)
				}
				return fmt.Errorf("query role %s failed: %w", key, err)
			}
			ur := &model.UserRole{UserID: userID, RoleID: role.ID, CreatedAt: time.Now()}
			if err := tx.WithContext(ctx).Create(ur).Error; err != nil {
				return fmt.Errorf("bind role %s failed: %w", key, err)
			}
		}
		return nil
	})
}

// GetRoleKeysForUser 查询用户的所有 role_key（启用状态）
func (s *UserRoleService) GetRoleKeysForUser(ctx context.Context, userID uint) ([]string, error) {
	var keys []string
	if err := s.db.WithContext(ctx).
		Table("sys_user_role").
		Select("r.role_key").
		Joins("JOIN sys_role r ON r.id = sys_user_role.role_id").
		Where("sys_user_role.user_id = ? AND r.status = 1", userID).
		Pluck("r.role_key", &keys).Error; err != nil {
		return nil, fmt.Errorf("query user role keys failed: %w", err)
	}
	return keys, nil
}

// PickHighestRole 从一组 role_key 里挑优先级最高的
// 用于 JWT AccessClaims.Role（单值）
// 优先级：admin > 自定义(system=0) > user > viewer
func PickHighestRole(roleKeys []string) string {
	if len(roleKeys) == 0 {
		return "user" // 兜底
	}
	for _, k := range roleKeys {
		if k == "admin" {
			return "admin"
		}
	}
	for _, k := range roleKeys {
		if k != "user" && k != "viewer" {
			return k
		}
	}
	for _, k := range roleKeys {
		if k == "user" {
			return "user"
		}
	}
	for _, k := range roleKeys {
		if k == "viewer" {
			return "viewer"
		}
	}
	return roleKeys[0]
}