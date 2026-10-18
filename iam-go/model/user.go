package model

import (
	"time"

	"gorm.io/gorm"
)

// User 用户表 model
// 对齐 DDL: sys_user（部门表 DDL 待后续补充）
type User struct {
	gorm.Model

	Username string `gorm:"column:username;size:64;uniqueIndex;not null" json:"username"`
	PasswordHash string `gorm:"column:password_hash;size:255;not null" json:"-"`

	RealName string `gorm:"column:real_name;size:64;not null" json:"real_name"`
	Email    string `gorm:"column:email;size:128;uniqueIndex" json:"email"`
	Phone    string `gorm:"column:phone;size:32;uniqueIndex" json:"phone"`
	AvatarURL string `gorm:"column:avatar_url;size:255" json:"avatar_url"`

	DepartmentID *uint64 `gorm:"column:department_id;index" json:"department_id"`

	Status int8 `gorm:"column:status;not null;default:1" json:"status"` // 1=启用 0=冻结

	LastLoginAt *time.Time `gorm:"column:last_login_at" json:"last_login_at"`
	LastLoginIP string     `gorm:"column:last_login_ip;size:64" json:"last_login_ip"`
}

func (User) TableName() string {
	return "sys_user"
}