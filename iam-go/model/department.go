package model

import (
	"gorm.io/gorm"
)

// Department 部门表 model
// 对齐 DDL: sys_department（自关联 parent_id，树形结构）
type Department struct {
	gorm.Model

	ParentID  uint   `gorm:"column:parent_id;not null;default:0;index" json:"parent_id"`
	Name      string `gorm:"column:name;size:128;not null" json:"name"`
	Code      string `gorm:"column:code;size:64;not null;uniqueIndex" json:"code"`
	Ancestors string `gorm:"column:ancestors;size:512" json:"ancestors"` // "0,1,5" 从根到当前
	LeaderID  *uint  `gorm:"column:leader_id;index" json:"leader_id"`
	SortOrder int    `gorm:"column:sort_order;not null;default:0" json:"sort_order"`
	Status    int8   `gorm:"column:status;not null;default:1" json:"status"` // 1=启用 0=停用

	// 树形返回时填充（GORM 不映射到 DB）
	Children []*Department `gorm:"-" json:"children,omitempty"`
}

func (Department) TableName() string {
	return "sys_department"
}