package model

import "time"

type RolePermission struct {
	ID        uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	RoleID    uint      `gorm:"column:role_id;not null;uniqueIndex:uk_role_permission" json:"role_id"`
	PermissionID uint    `gorm:"column:permission_id;not null;uniqueIndex:uk_role_permission;index" json:"permission_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (RolePermission) TableName() string {
	return "sys_role_permission"
}