package model

import (
	"gorm.io/gorm"
)

// Role 角色表 model
// 对齐 DDL: sys_role
type Role struct {
	gorm.Model

	RoleKey     string `gorm:"column:role_key;size:64;not null;uniqueIndex" json:"role_key"`
	RoleName    string `gorm:"column:role_name;size:128;not null" json:"role_name"`
	Description string `gorm:"column:description;size:255" json:"description"`
	Status      int8   `gorm:"column:status;not null;default:1" json:"status"`   // 1=启用 0=停用
	IsSystem    int8   `gorm:"column:is_system;not null;default:0" json:"is_system"` // 1=系统内置 0=自定义
}

func (Role) TableName() string {
	return "sys_role"
}