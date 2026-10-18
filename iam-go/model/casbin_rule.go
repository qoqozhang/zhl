package model

type CasbinRule struct {
	ID    uint   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	PType string `gorm:"column:ptype;size:100;not null;index" json:"ptype"`
	V0    string `gorm:"column:v0;size:100;index" json:"v0"`
	V1    string `gorm:"column:v1;size:100" json:"v1"`
	V2    string `gorm:"column:v2;size:100" json:"v2"`
	V3    string `gorm:"column:v3;size:100" json:"v3,omitempty"`
	V4    string `gorm:"column:v4;size:100" json:"v4,omitempty"`
	V5    string `gorm:"column:v5;size:100" json:"v5,omitempty"`
}

func (CasbinRule) TableName() string {
	return "casbin_rule"
}