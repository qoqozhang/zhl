package model

import "time"

// UserRole 用户-角色关联表 model
// 对齐 DDL: sys_user_role
type UserRole struct {
	ID        uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    uint      `gorm:"column:user_id;not null;uniqueIndex:uk_user_role" json:"user_id"`
	RoleID    uint      `gorm:"column:role_id;not null;uniqueIndex:uk_user_role;index" json:"role_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (UserRole) TableName() string {
	return "sys_user_role"
}