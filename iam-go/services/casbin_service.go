package services

import (
	"context"
	"fmt"

	"iam-go/model"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"
)

type CasbinService struct {
	db       *gorm.DB
	enforcer *casbin.Enforcer
}

func NewCasbinService(db *gorm.DB, modelPath string) (*CasbinService, error) {
	e, err := casbin.NewEnforcer(modelPath)
	if err != nil {
		return nil, fmt.Errorf("init casbin enforcer failed: %w", err)
	}
	return &CasbinService{db: db, enforcer: e}, nil
}

func (s *CasbinService) Enforce(role, path, method string) (bool, error) {
	return s.enforcer.Enforce(role, path, method)
}

func (s *CasbinService) Enforcer() *casbin.Enforcer {
	return s.enforcer
}

// EnsureStartupPolicy 启动时确保 casbin_rule 有数据（幂等）
func (s *CasbinService) EnsureStartupPolicy(ctx context.Context) error {
	var count int64
	s.db.WithContext(ctx).Model(&model.CasbinRule{}).Count(&count)
	if count == 0 {
		return s.SyncAllRules(ctx)
	}
	return s.LoadFromDB(ctx)
}

// LoadFromDB 从 casbin_rule 表全量加载到 Enforcer 内存
func (s *CasbinService) LoadFromDB(ctx context.Context) error {
	var rules []*model.CasbinRule
	if err := s.db.WithContext(ctx).Find(&rules).Error; err != nil {
		return fmt.Errorf("query casbin rules failed: %w", err)
	}

	s.enforcer.ClearPolicy()

	for _, r := range rules {
		switch r.PType {
		case "p":
			_, _ = s.enforcer.AddPolicy(policyFields(r))
		case "g":
			_, _ = s.enforcer.AddGroupingPolicy(groupFields(r))
		}
	}
	return nil
}

func policyFields(r *model.CasbinRule) []string {
	fields := []string{r.V0, r.V1, r.V2}
	for _, v := range []string{r.V3, r.V4, r.V5} {
		if v != "" {
			fields = append(fields, v)
		}
	}
	return fields
}

func groupFields(r *model.CasbinRule) []string {
	fields := []string{r.V0, r.V1}
	for _, v := range []string{r.V2, r.V3, r.V4, r.V5} {
		if v != "" {
			fields = append(fields, v)
		}
	}
	return fields
}

// SyncAllRules 从 sys_role_permission + sys_permission + sys_role 重建 casbin_rule
func (s *CasbinService) SyncAllRules(ctx context.Context) error {
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}

	if err := tx.Delete(&model.CasbinRule{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("clear casbin_rule failed: %w", err)
	}

	// 角色继承 g, child, parent
	inheritance := [][]string{
		{"admin", "user"},
		{"user", "viewer"},
	}
	for _, rel := range inheritance {
		if err := tx.Create(&model.CasbinRule{
			PType: "g", V0: rel[0], V1: rel[1],
		}).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	type rpJoin struct {
		RoleKey    string `gorm:"column:role_key"`
		APIPath    string `gorm:"column:api_path"`
		HTTPMethod string `gorm:"column:http_method"`
	}

	var rows []rpJoin
	if err := tx.Raw(`
		SELECT DISTINCT r.role_key, p.api_path, p.http_method
		FROM sys_role_permission rp
		JOIN sys_role r ON rp.role_id = r.id
		JOIN sys_permission p ON rp.permission_id = p.id
		WHERE p.api_path IS NOT NULL
		  AND p.api_path != ''
		  AND p.http_method IS NOT NULL
		  AND p.http_method != ''
	`).Scan(&rows).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("query role-permission mapping failed: %w", err)
	}

	for _, row := range rows {
		if row.RoleKey == "" || row.APIPath == "" || row.HTTPMethod == "" {
			continue
		}
		if err := tx.Create(&model.CasbinRule{
			PType: "p", V0: row.RoleKey, V1: row.APIPath, V2: row.HTTPMethod,
		}).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	return s.LoadFromDB(ctx)
}