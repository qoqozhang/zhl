package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"iam-go/model"

	"gorm.io/gorm"
)

type CasbinSyncer interface {
	SyncAllRules(ctx context.Context) error
}

type RolePermissionService struct {
	db      *gorm.DB
	syncer  CasbinSyncer
}

func NewRolePermissionService(db *gorm.DB) *RolePermissionService {
	return &RolePermissionService{db: db}
}

func (s *RolePermissionService) WithSyncer(syncer CasbinSyncer) *RolePermissionService {
	s.syncer = syncer
	return s
}

func (s *RolePermissionService) afterChange(ctx context.Context) {
	if s.syncer != nil {
		_ = s.syncer.SyncAllRules(ctx)
	}
}

func (s *RolePermissionService) Bind(ctx context.Context, roleID uint, permissionID uint) error {
	var count int64
	s.db.WithContext(ctx).Model(&model.RolePermission{}).
		Where("role_id = ? AND permission_id = ?", roleID, permissionID).Count(&count)
	if count > 0 {
		return nil
	}
	rp := &model.RolePermission{RoleID: roleID, PermissionID: permissionID, CreatedAt: time.Now()}
	if err := s.db.WithContext(ctx).Create(rp).Error; err != nil {
		return fmt.Errorf("bind role-permission failed: %w", err)
	}
	s.afterChange(ctx)
	return nil
}

func (s *RolePermissionService) Unbind(ctx context.Context, roleID uint, permissionID uint) error {
	if err := s.db.WithContext(ctx).
		Where("role_id = ? AND permission_id = ?", roleID, permissionID).
		Delete(&model.RolePermission{}).Error; err != nil {
		return fmt.Errorf("unbind role-permission failed: %w", err)
	}
	s.afterChange(ctx)
	return nil
}

// ReplacePermissions 用一组 permission_ids 完全替换角色的权限绑定
func (s *RolePermissionService) ReplacePermissions(ctx context.Context, roleID uint, permissionIDs []uint) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&model.RolePermission{}).Error; err != nil {
			return fmt.Errorf("clear old permissions failed: %w", err)
		}
		for _, pid := range permissionIDs {
			rp := &model.RolePermission{RoleID: roleID, PermissionID: pid, CreatedAt: time.Now()}
			if err := tx.WithContext(ctx).Create(rp).Error; err != nil {
				return fmt.Errorf("bind permission %d failed: %w", pid, err)
			}
		}
		return nil
	})
	if err == nil {
		s.afterChange(ctx)
	}
	return err
}

// ReplaceByKeys 用 permission_key 列表替换角色权限绑定
func (s *RolePermissionService) ReplaceByKeys(ctx context.Context, roleID uint, permissionKeys []string) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&model.RolePermission{}).Error; err != nil {
			return fmt.Errorf("clear old permissions failed: %w", err)
		}
		for _, key := range permissionKeys {
			var perm model.Permission
			if err := tx.WithContext(ctx).Where("permission_key = ?", key).First(&perm).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return fmt.Errorf("query permission %s failed: %w", key, err)
			}
			rp := &model.RolePermission{RoleID: roleID, PermissionID: perm.ID, CreatedAt: time.Now()}
			if err := tx.WithContext(ctx).Create(rp).Error; err != nil {
				return fmt.Errorf("bind permission %s failed: %w", key, err)
			}
		}
		return nil
	})
	if err == nil {
		s.afterChange(ctx)
	}
	return err
}

// GetPermissionKeysForRole 查询角色的所有 permission_key（启用状态）
func (s *RolePermissionService) GetPermissionKeysForRole(ctx context.Context, roleID uint) ([]string, error) {
	var keys []string
	if err := s.db.WithContext(ctx).
		Table("sys_role_permission").
		Select("p.permission_key").
		Joins("JOIN sys_permission p ON p.id = sys_role_permission.permission_id").
		Where("sys_role_permission.role_id = ? AND p.status = 1", roleID).
		Pluck("p.permission_key", &keys).Error; err != nil {
		return nil, fmt.Errorf("query role permission keys failed: %w", err)
	}
	return keys, nil
}

// GetPermissionIDsForRole 查询角色绑定的所有 permission_id
func (s *RolePermissionService) GetPermissionIDsForRole(ctx context.Context, roleID uint) ([]uint, error) {
	var ids []uint
	if err := s.db.WithContext(ctx).
		Model(&model.RolePermission{}).
		Where("role_id = ?", roleID).
		Pluck("permission_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("query role permission ids failed: %w", err)
	}
	return ids, nil
}

// HasPermission 检查 role_key 是否拥有 permission_key 权限
// 用于 PermissionMiddleware 动态鉴权
func (s *RolePermissionService) HasPermission(ctx context.Context, roleKey string, permissionKey string) (bool, error) {
	if roleKey == "admin" {
		return true, nil
	}
	var count int64
	err := s.db.WithContext(ctx).
		Table("sys_role_permission").
		Joins("JOIN sys_role r ON r.id = sys_role_permission.role_id AND r.role_key = ? AND r.status = 1", roleKey).
		Joins("JOIN sys_permission p ON p.id = sys_role_permission.permission_id AND p.permission_key = ? AND p.status = 1", permissionKey).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check permission failed: %w", err)
	}
	return count > 0, nil
}