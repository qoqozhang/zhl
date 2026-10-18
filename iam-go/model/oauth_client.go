package model

import "time"

type OAuthClient struct {
	ID             uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ClientID       string    `gorm:"column:client_id;size:128;not null;uniqueIndex" json:"client_id"`
	ClientSecretHash string  `gorm:"column:client_secret_hash;size:255;not null" json:"-"`
	ClientName     string    `gorm:"column:client_name;size:128;not null" json:"client_name"`
	RedirectURIs   string    `gorm:"column:redirect_uris;type:text;not null" json:"redirect_uris"`
	Scopes         string    `gorm:"column:scopes;size:512" json:"scopes"`
	Status         int8      `gorm:"column:status;not null;default:1" json:"status"`
	CreatedAt      time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (OAuthClient) TableName() string {
	return "oauth_client"
}

func (c *OAuthClient) ParseRedirectURIs() []string {
	if c == nil || c.RedirectURIs == "" {
		return nil
	}
	result := []string{}
	current := ""
	for _, ch := range c.RedirectURIs {
		if ch == ',' {
			if current != "" {
				result = append(result, current)
			}
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func (c *OAuthClient) ValidateRedirectURI(uri string) bool {
	for _, u := range c.ParseRedirectURIs() {
		if u == uri {
			return true
		}
	}
	return false
}