package model

import "time"

type OAuthAuthorizationCode struct {
	ID          uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CodeHash    string    `gorm:"column:code_hash;size:255;not null;uniqueIndex" json:"-"`
	ClientID    string    `gorm:"column:client_id;size:128;not null;index" json:"client_id"`
	UserID      uint      `gorm:"column:user_id;not null;index" json:"user_id"`
	RedirectURI string    `gorm:"column:redirect_uri;size:512;not null" json:"redirect_uri"`
	Scopes      string    `gorm:"column:scopes;size:512" json:"scopes"`
	ExpiresAt   time.Time `gorm:"column:expires_at;not null;index" json:"expires_at"`
	Used        int8      `gorm:"column:used;not null;default:0" json:"used"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (OAuthAuthorizationCode) TableName() string {
	return "oauth_authorization_code"
}