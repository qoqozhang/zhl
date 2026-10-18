package model

import "gorm.io/gorm"

type Permission struct {
	gorm.Model

	ParentID      uint   `gorm:"column:parent_id;not null;default:0;index" json:"parent_id"`
	Name          string `gorm:"column:name;size:128;not null" json:"name"`
	PermissionKey string `gorm:"column:permission_key;size:128;not null;uniqueIndex" json:"permission_key"`
	ResourceType  string `gorm:"column:resource_type;size:32;not null;index" json:"resource_type"`
	Path          string `gorm:"column:path;size:255" json:"path"`
	HTTPMethod    string `gorm:"column:http_method;size:16" json:"http_method"`
	Icon          string `gorm:"column:icon;size:64" json:"icon"`
	SortOrder     int    `gorm:"column:sort_order;not null;default:0" json:"sort_order"`
	Visible       int8   `gorm:"column:visible;not null;default:1" json:"visible"`
	Status        int8   `gorm:"column:status;not null;default:1" json:"status"`

	Children []*Permission `gorm:"-" json:"children,omitempty"`
}

func (Permission) TableName() string {
	return "sys_permission"
}