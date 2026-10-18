package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"iam-go/model"

	"gorm.io/gorm"
)

var (
	ErrDepartmentNotFound    = errors.New("department not found")
	ErrDepartmentExists      = errors.New("department code already exists")
	ErrDepartmentHasChildren = errors.New("department has children, cannot delete")
	ErrDepartmentHasUsers    = errors.New("department has users, cannot delete")
	ErrDepartmentParentCycle = errors.New("parent_id cannot be itself or its descendant")
)

type DepartmentService struct {
	db *gorm.DB
}

func NewDepartmentService(db *gorm.DB) *DepartmentService {
	return &DepartmentService{db: db}
}

type DepartmentCreateRequest struct {
	ParentID  uint   `json:"parent_id"  binding:"omitempty,min=0"`
	Name      string `json:"name"       binding:"required,max=128"`
	Code      string `json:"code"       binding:"required,max=64"`
	LeaderID  *uint  `json:"leader_id"  binding:"omitempty"`
	SortOrder int    `json:"sort_order" binding:"omitempty"`
	Status    *int8  `json:"status"     binding:"omitempty,oneof=0 1"`
}

type DepartmentUpdateRequest struct {
	ParentID  *uint  `json:"parent_id"  binding:"omitempty,min=0"`
	Name      *string `json:"name"      binding:"omitempty,max=128"`
	Code      *string `json:"code"      binding:"omitempty,max=64"`
	LeaderID  *uint  `json:"leader_id"  binding:"omitempty"`
	SortOrder *int   `json:"sort_order" binding:"omitempty"`
	Status    *int8  `json:"status"     binding:"omitempty,oneof=0 1"`
}

type DepartmentListParams struct {
	ParentID *uint  `form:"parent_id" binding:"omitempty"`
	Status   *int8  `form:"status"    binding:"omitempty,oneof=0 1"`
	Keyword  string `form:"keyword"`
	Tree     bool   `form:"tree"`
}

type DepartmentListResult struct {
	Total int64              `json:"total"`
	Items []model.Department `json:"items"`
}

func (s *DepartmentService) Create(ctx context.Context, req *DepartmentCreateRequest) (*model.Department, error) {
	var count int64
	s.db.WithContext(ctx).Model(&model.Department{}).Where("code = ?", req.Code).Count(&count)
	if count > 0 {
		return nil, ErrDepartmentExists
	}

	var ancestors string
	parentID := req.ParentID
	if parentID != 0 {
		var parent model.Department
		if err := s.db.WithContext(ctx).First(&parent, parentID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrDepartmentNotFound
			}
			return nil, fmt.Errorf("query parent failed: %w", err)
		}
		ancestors = parent.Ancestors
		if ancestors == "" {
			ancestors = fmt.Sprintf("%d", parent.ID)
		} else {
			ancestors = ancestors + "," + fmt.Sprintf("%d", parent.ID)
		}
	}

	if req.LeaderID != nil {
		var userCount int64
		s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", *req.LeaderID).Count(&userCount)
		if userCount == 0 {
			return nil, fmt.Errorf("leader user not found: %d", *req.LeaderID)
		}
	}

	status := int8(1)
	if req.Status != nil {
		status = *req.Status
	}

	u := &model.Department{
		ParentID:  parentID,
		Name:      req.Name,
		Code:      req.Code,
		Ancestors: ancestors,
		LeaderID:  req.LeaderID,
		SortOrder: req.SortOrder,
		Status:    status,
	}
	if err := s.db.WithContext(ctx).Create(u).Error; err != nil {
		return nil, fmt.Errorf("create department failed: %w", err)
	}
	return u, nil
}

func (s *DepartmentService) GetByID(ctx context.Context, id uint) (*model.Department, error) {
	var d model.Department
	if err := s.db.WithContext(ctx).First(&d, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDepartmentNotFound
		}
		return nil, fmt.Errorf("query department failed: %w", err)
	}
	return &d, nil
}

func (s *DepartmentService) List(ctx context.Context, params *DepartmentListParams) (interface{}, error) {
	var items []model.Department
	var err error

	if params.Tree {
		q := s.db.WithContext(ctx).Model(&model.Department{})
		if params.Status != nil {
			q = q.Where("status = ?", *params.Status)
		}
		if params.Keyword != "" {
			like := "%" + params.Keyword + "%"
			q = q.Where("name LIKE ? OR code LIKE ?", like, like)
		}
		err = q.Order("sort_order ASC, id ASC").Find(&items).Error
		if err != nil {
			return nil, fmt.Errorf("list departments failed: %w", err)
		}
		tree := buildDepartmentTree(items)
		return tree, nil
	}

	q := s.db.WithContext(ctx).Model(&model.Department{})
	if params.ParentID != nil {
		q = q.Where("parent_id = ?", *params.ParentID)
	}
	if params.Status != nil {
		q = q.Where("status = ?", *params.Status)
	}
	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		q = q.Where("name LIKE ? OR code LIKE ?", like, like)
	}
	err = q.Order("sort_order ASC, id ASC").Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("list departments failed: %w", err)
	}

	var total int64
	s.db.WithContext(ctx).Model(&model.Department{}).Count(&total)
	return &DepartmentListResult{Total: total, Items: items}, nil
}

func (s *DepartmentService) Update(ctx context.Context, id uint, req *DepartmentUpdateRequest) (*model.Department, error) {
	d, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}

	if req.ParentID != nil {
		if *req.ParentID == id {
			return nil, ErrDepartmentParentCycle
		}
		if *req.ParentID != 0 {
			childrenIDs, err := s.getAllDescendantIDs(ctx, id)
			if err != nil {
				return nil, err
			}
			childrenIDs = append(childrenIDs, id)
			for _, cid := range childrenIDs {
				if cid == *req.ParentID {
					return nil, ErrDepartmentParentCycle
				}
			}

			var parent model.Department
			if err := s.db.WithContext(ctx).First(&parent, *req.ParentID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, ErrDepartmentNotFound
				}
				return nil, fmt.Errorf("query new parent failed: %w", err)
			}
			newAncestors := parent.Ancestors
			if newAncestors == "" {
				newAncestors = fmt.Sprintf("%d", parent.ID)
			} else {
				newAncestors = newAncestors + "," + fmt.Sprintf("%d", parent.ID)
			}
			updates["ancestors"] = newAncestors
		} else {
			updates["ancestors"] = ""
		}
		updates["parent_id"] = *req.ParentID
	}

	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Code != nil {
		var count int64
		s.db.WithContext(ctx).Model(&model.Department{}).
			Where("code = ? AND id != ?", *req.Code, id).Count(&count)
		if count > 0 {
			return nil, ErrDepartmentExists
		}
		updates["code"] = *req.Code
	}
	if req.LeaderID != nil {
		updates["leader_id"] = *req.LeaderID
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if len(updates) == 0 {
		return d, nil
	}

	if err := s.db.WithContext(ctx).Model(d).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update department failed: %w", err)
	}

	if req.ParentID != nil {
		s.updateDescendantsAncestors(ctx, id)
	}

	return s.GetByID(ctx, id)
}

func (s *DepartmentService) Delete(ctx context.Context, id uint) error {
	d, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	var childCount int64
	s.db.WithContext(ctx).Model(&model.Department{}).Where("parent_id = ?", id).Count(&childCount)
	if childCount > 0 {
		return ErrDepartmentHasChildren
	}

	var userCount int64
	s.db.WithContext(ctx).Model(&model.User{}).Where("department_id = ?", id).Count(&userCount)
	if userCount > 0 {
		return ErrDepartmentHasUsers
	}

	if err := s.db.WithContext(ctx).Delete(d).Error; err != nil {
		return fmt.Errorf("delete department failed: %w", err)
	}
	return nil
}

func (s *DepartmentService) getAllDescendantIDs(ctx context.Context, id uint) ([]uint, error) {
	var ids []uint
	like := "%" + fmt.Sprintf("%d", id) + "%"
	if err := s.db.WithContext(ctx).Model(&model.Department{}).
		Where("ancestors LIKE ?", like).Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("query descendants failed: %w", err)
	}
	return ids, nil
}

func (s *DepartmentService) updateDescendantsAncestors(ctx context.Context, id uint) {
	var descendants []model.Department
	s.db.WithContext(ctx).Model(&model.Department{}).Where("ancestors LIKE ?", "%"+fmt.Sprintf("%d", id)+"%").Find(&descendants)

	var self model.Department
	if err := s.db.WithContext(ctx).First(&self, id).Error; err != nil {
		return
	}

	for _, child := range descendants {
		s.db.WithContext(ctx).Model(&child).Update("ancestors", self.Ancestors)
	}
}

func buildDepartmentTree(items []model.Department) []*model.Department {
	idMap := make(map[uint]*model.Department, len(items))
	for i := range items {
		items[i].Children = nil
		idMap[items[i].ID] = &items[i]
	}

	var roots []*model.Department
	for i := range items {
		node := idMap[items[i].ID]
		if node.ParentID == 0 {
			roots = append(roots, node)
			continue
		}
		if parent, ok := idMap[node.ParentID]; ok {
			parent.Children = append(parent.Children, node)
		} else {
			roots = append(roots, node)
		}
	}

	sortDepartments(roots)
	return roots
}

func sortDepartments(nodes []*model.Department) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].SortOrder != nodes[j].SortOrder {
			return nodes[i].SortOrder < nodes[j].SortOrder
		}
		return nodes[i].ID < nodes[j].ID
	})
	for _, n := range nodes {
		if len(n.Children) > 0 {
			sortDepartments(n.Children)
		}
	}
}

func parseAncestors(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		if p != "0" && p != "" {
			result = append(result, p)
		}
	}
	return result
}