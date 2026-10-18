package services

import (
	"context"
	"errors"
	"fmt"

	"iam-go/model"

	"gorm.io/gorm"
)

var (
	ErrPermissionNotFound     = errors.New("permission not found")
	ErrPermissionExists       = errors.New("permission key already exists")
	ErrPermissionParentCycle = errors.New("parent cannot be self or its descendant")
	ErrPermissionHasChildren = errors.New("cannot delete permission with children")
	ErrPermissionInUse       = errors.New("permission is assigned to roles and cannot be deleted")
)

type PermissionService struct {
	db *gorm.DB
}

func NewPermissionService(db *gorm.DB) *PermissionService {
	return &PermissionService{db: db}
}

type PermissionCreateRequest struct {
	ParentID      uint   `json:"parent_id"      binding:"omitempty,min=0"`
	Name          string `json:"name"           binding:"required,max=128"`
	PermissionKey string `json:"permission_key" binding:"required,max=128"`
	ResourceType  string `json:"resource_type"  binding:"required,oneof=menu page button api"`
	Path          string `json:"path"           binding:"omitempty,max=255"`
	HTTPMethod    string `json:"http_method"    binding:"omitempty,oneof=GET POST PUT DELETE PATCH HEAD OPTIONS"`
	Icon          string `json:"icon"           binding:"omitempty,max=64"`
	SortOrder     int    `json:"sort_order"     binding:"omitempty"`
	Visible       *int8  `json:"visible"        binding:"omitempty,oneof=0 1"`
	Status        *int8  `json:"status"         binding:"omitempty,oneof=0 1"`
}

type PermissionUpdateRequest struct {
	ParentID      *uint   `json:"parent_id"      binding:"omitempty,min=0"`
	Name          *string `json:"name"           binding:"omitempty,max=128"`
	PermissionKey *string `json:"permission_key" binding:"omitempty,max=128"`
	ResourceType  *string `json:"resource_type"  binding:"omitempty,oneof=menu page button api"`
	Path          *string `json:"path"           binding:"omitempty,max=255"`
	HTTPMethod    *string `json:"http_method"    binding:"omitempty,oneof=GET POST PUT DELETE PATCH HEAD OPTIONS"`
	Icon          *string `json:"icon"           binding:"omitempty,max=64"`
	SortOrder     *int    `json:"sort_order"     binding:"omitempty"`
	Visible       *int8   `json:"visible"        binding:"omitempty,oneof=0 1"`
	Status        *int8   `json:"status"         binding:"omitempty,oneof=0 1"`
}

type PermissionListParams struct {
	ResourceType *string `form:"resource_type" binding:"omitempty,oneof=menu page button api"`
	Status       *int8   `form:"status"        binding:"omitempty,oneof=0 1"`
	ParentID     *uint   `form:"parent_id"`
	Keyword      string  `form:"keyword"`
	Tree         bool    `form:"tree"`
}

type PermissionListResult struct {
	Total int64              `json:"total"`
	Items []*model.Permission `json:"items"`
}

func (s *PermissionService) Create(ctx context.Context, req *PermissionCreateRequest) (*model.Permission, error) {
	var count int64
	s.db.WithContext(ctx).Model(&model.Permission{}).
		Where("permission_key = ?", req.PermissionKey).Count(&count)
	if count > 0 {
		return nil, ErrPermissionExists
	}

	if req.ParentID > 0 {
		var parentCount int64
		s.db.WithContext(ctx).Model(&model.Permission{}).Where("id = ?", req.ParentID).Count(&parentCount)
		if parentCount == 0 {
			return nil, fmt.Errorf("parent permission not found: %d", req.ParentID)
		}
	}

	visible := int8(1)
	if req.Visible != nil {
		visible = *req.Visible
	}
	status := int8(1)
	if req.Status != nil {
		status = *req.Status
	}

	p := &model.Permission{
		ParentID:      req.ParentID,
		Name:          req.Name,
		PermissionKey: req.PermissionKey,
		ResourceType:  req.ResourceType,
		Path:          req.Path,
		HTTPMethod:    req.HTTPMethod,
		Icon:          req.Icon,
		SortOrder:     req.SortOrder,
		Visible:       visible,
		Status:        status,
	}
	if err := s.db.WithContext(ctx).Create(p).Error; err != nil {
		return nil, fmt.Errorf("create permission failed: %w", err)
	}
	return p, nil
}

func (s *PermissionService) GetByID(ctx context.Context, id uint) (*model.Permission, error) {
	var p model.Permission
	if err := s.db.WithContext(ctx).First(&p, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPermissionNotFound
		}
		return nil, fmt.Errorf("query permission failed: %w", err)
	}
	return &p, nil
}

func (s *PermissionService) List(ctx context.Context, params *PermissionListParams) (*PermissionListResult, error) {
	q := s.db.WithContext(ctx).Model(&model.Permission{})

	if params.ResourceType != nil {
		q = q.Where("resource_type = ?", *params.ResourceType)
	}
	if params.Status != nil {
		q = q.Where("status = ?", *params.Status)
	}
	if params.ParentID != nil {
		q = q.Where("parent_id = ?", *params.ParentID)
	}
	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		q = q.Where("name LIKE ? OR permission_key LIKE ? OR path LIKE ?", like, like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count permissions failed: %w", err)
	}

	var items []*model.Permission
	if err := q.Order("sort_order ASC, id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list permissions failed: %w", err)
	}

	if params.Tree {
		return &PermissionListResult{
			Total: total,
			Items: buildPermissionTree(items, 0),
		}, nil
	}

	return &PermissionListResult{Total: total, Items: items}, nil
}

func buildPermissionTree(items []*model.Permission, rootParentID uint) []*model.Permission {
	byParent := make(map[uint][]*model.Permission)
	for _, p := range items {
		byParent[p.ParentID] = append(byParent[p.ParentID], p)
	}
	var build func(parentID uint) []*model.Permission
	build = func(parentID uint) []*model.Permission {
		children := byParent[parentID]
		for _, c := range children {
			c.Children = build(c.ID)
		}
		return children
	}
	return build(rootParentID)
}

func (s *PermissionService) Update(ctx context.Context, id uint, req *PermissionUpdateRequest) (*model.Permission, error) {
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}

	if req.ParentID != nil {
		if *req.ParentID == id {
			return nil, ErrPermissionParentCycle
		}
		if *req.ParentID > 0 {
			descendants, err := s.getAllDescendants(ctx, id)
			if err != nil {
				return nil, err
			}
			for _, d := range descendants {
				if d == *req.ParentID {
					return nil, ErrPermissionParentCycle
				}
			}
		}
		updates["parent_id"] = *req.ParentID
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.PermissionKey != nil {
		var count int64
		s.db.WithContext(ctx).Model(&model.Permission{}).
			Where("permission_key = ? AND id != ?", *req.PermissionKey, id).Count(&count)
		if count > 0 {
			return nil, ErrPermissionExists
		}
		updates["permission_key"] = *req.PermissionKey
	}
	if req.ResourceType != nil {
		updates["resource_type"] = *req.ResourceType
	}
	if req.Path != nil {
		updates["path"] = *req.Path
	}
	if req.HTTPMethod != nil {
		updates["http_method"] = *req.HTTPMethod
	}
	if req.Icon != nil {
		updates["icon"] = *req.Icon
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if req.Visible != nil {
		updates["visible"] = *req.Visible
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if len(updates) == 0 {
		return p, nil
	}

	if err := s.db.WithContext(ctx).Model(p).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update permission failed: %w", err)
	}

	return s.GetByID(ctx, id)
}

func (s *PermissionService) getAllDescendants(ctx context.Context, id uint) ([]uint, error) {
	var all []uint
	queue := []uint{id}
	for len(queue) > 0 {
		var children []model.Permission
		if err := s.db.WithContext(ctx).
			Where("parent_id IN ?", queue).
			Find(&children).Error; err != nil {
			return nil, fmt.Errorf("query descendants failed: %w", err)
		}
		queue = nil
		for _, c := range children {
			all = append(all, c.ID)
			queue = append(queue, c.ID)
		}
	}
	return all, nil
}

func (s *PermissionService) Delete(ctx context.Context, id uint) error {
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	var childCount int64
	s.db.WithContext(ctx).Model(&model.Permission{}).Where("parent_id = ?", id).Count(&childCount)
	if childCount > 0 {
		return ErrPermissionHasChildren
	}

	var rolePermCount int64
	s.db.WithContext(ctx).Table("sys_role_permission").Where("permission_id = ?", id).Count(&rolePermCount)
	if rolePermCount > 0 {
		return ErrPermissionInUse
	}

	if err := s.db.WithContext(ctx).Delete(p).Error; err != nil {
		return fmt.Errorf("delete permission failed: %w", err)
	}
	return nil
}

// MatchPermissionKey 根据 Gin FullPath + HTTP Method 匹配 permission_key
// 用于 PermissionMiddleware 动态鉴权
// 返回空字符串表示路由没有配置 permission（放行）
func (s *PermissionService) MatchPermissionKey(ctx context.Context, fullPath string, method string) (string, error) {
	var p model.Permission
	err := s.db.WithContext(ctx).
		Where("path = ? AND http_method = ? AND status = 1", fullPath, method).
		First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", fmt.Errorf("match permission failed: %w", err)
	}
	return p.PermissionKey, nil
}

// EnsureDefaultPermissions 启动时写入种子权限数据（幂等）
func (s *PermissionService) EnsureDefaultPermissions(ctx context.Context) error {
	defaults := []*model.Permission{
		{ParentID: 0, Name: "用户管理", PermissionKey: "user", ResourceType: "menu", Path: "/users", Icon: "User", SortOrder: 100, Visible: 1, Status: 1},
		{ParentID: 0, Name: "部门管理", PermissionKey: "department", ResourceType: "menu", Path: "/departments", Icon: "OfficeBuilding", SortOrder: 110, Visible: 1, Status: 1},
		{ParentID: 0, Name: "角色管理", PermissionKey: "role", ResourceType: "menu", Path: "/roles", Icon: "UserFilled", SortOrder: 120, Visible: 1, Status: 1},
		{ParentID: 0, Name: "权限管理", PermissionKey: "permission", ResourceType: "menu", Path: "/permissions", Icon: "Lock", SortOrder: 130, Visible: 1, Status: 1},

		{ParentID: 0, Name: "用户-登录", PermissionKey: "user_login", ResourceType: "api", Path: "/v1/users/login", HTTPMethod: "POST", SortOrder: 200, Status: 1},
		{ParentID: 0, Name: "用户-刷新Token", PermissionKey: "user_refresh", ResourceType: "api", Path: "/v1/users/refresh", HTTPMethod: "POST", SortOrder: 201, Status: 1},
		{ParentID: 0, Name: "用户-登出", PermissionKey: "user_logout", ResourceType: "api", Path: "/v1/users/logout", HTTPMethod: "POST", SortOrder: 202, Status: 1},
		{ParentID: 0, Name: "用户-查询自己", PermissionKey: "user_me", ResourceType: "api", Path: "/v1/users/me", HTTPMethod: "GET", SortOrder: 203, Status: 1},
		{ParentID: 0, Name: "用户-列表", PermissionKey: "user_list", ResourceType: "api", Path: "/v1/users", HTTPMethod: "GET", SortOrder: 204, Status: 1},
		{ParentID: 0, Name: "用户-创建", PermissionKey: "user_create", ResourceType: "api", Path: "/v1/users", HTTPMethod: "POST", SortOrder: 205, Status: 1},
		{ParentID: 0, Name: "用户-修改", PermissionKey: "user_update", ResourceType: "api", Path: "/v1/users/:id", HTTPMethod: "PUT", SortOrder: 206, Status: 1},
		{ParentID: 0, Name: "用户-删除", PermissionKey: "user_delete", ResourceType: "api", Path: "/v1/users/:id", HTTPMethod: "DELETE", SortOrder: 207, Status: 1},

		{ParentID: 0, Name: "部门-列表", PermissionKey: "department_list", ResourceType: "api", Path: "/v1/departments", HTTPMethod: "GET", SortOrder: 220, Status: 1},
		{ParentID: 0, Name: "部门-创建", PermissionKey: "department_create", ResourceType: "api", Path: "/v1/departments", HTTPMethod: "POST", SortOrder: 221, Status: 1},
		{ParentID: 0, Name: "部门-修改", PermissionKey: "department_update", ResourceType: "api", Path: "/v1/departments/:id", HTTPMethod: "PUT", SortOrder: 222, Status: 1},
		{ParentID: 0, Name: "部门-删除", PermissionKey: "department_delete", ResourceType: "api", Path: "/v1/departments/:id", HTTPMethod: "DELETE", SortOrder: 223, Status: 1},

		{ParentID: 0, Name: "角色-列表", PermissionKey: "role_list", ResourceType: "api", Path: "/v1/roles", HTTPMethod: "GET", SortOrder: 240, Status: 1},
		{ParentID: 0, Name: "角色-创建", PermissionKey: "role_create", ResourceType: "api", Path: "/v1/roles", HTTPMethod: "POST", SortOrder: 241, Status: 1},
		{ParentID: 0, Name: "角色-修改", PermissionKey: "role_update", ResourceType: "api", Path: "/v1/roles/:id", HTTPMethod: "PUT", SortOrder: 242, Status: 1},
		{ParentID: 0, Name: "角色-删除", PermissionKey: "role_delete", ResourceType: "api", Path: "/v1/roles/:id", HTTPMethod: "DELETE", SortOrder: 243, Status: 1},

		{ParentID: 0, Name: "权限-列表", PermissionKey: "permission_list", ResourceType: "api", Path: "/v1/permissions", HTTPMethod: "GET", SortOrder: 260, Status: 1},
		{ParentID: 0, Name: "权限-创建", PermissionKey: "permission_create", ResourceType: "api", Path: "/v1/permissions", HTTPMethod: "POST", SortOrder: 261, Status: 1},
		{ParentID: 0, Name: "权限-修改", PermissionKey: "permission_update", ResourceType: "api", Path: "/v1/permissions/:id", HTTPMethod: "PUT", SortOrder: 262, Status: 1},
		{ParentID: 0, Name: "权限-删除", PermissionKey: "permission_delete", ResourceType: "api", Path: "/v1/permissions/:id", HTTPMethod: "DELETE", SortOrder: 263, Status: 1},
	}

	for _, dp := range defaults {
		var count int64
		s.db.WithContext(ctx).Model(&model.Permission{}).
			Where("permission_key = ?", dp.PermissionKey).Count(&count)
		if count > 0 {
			continue
		}
		if err := s.db.WithContext(ctx).Create(dp).Error; err != nil {
			return fmt.Errorf("create default permission %s failed: %w", dp.PermissionKey, err)
		}
	}
	return nil
}