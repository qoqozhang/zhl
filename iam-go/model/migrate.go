package model

import (
	"iam-go/pkg/auditlog"
	"iam-go/pkg/logger"

	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) error {
	logger.Info().Msg("running auto migrate...")
	return db.AutoMigrate(
		&User{},
		&Department{},
		&Role{},
		&UserRole{},
		&Permission{},
		&RolePermission{},
		&OAuthClient{},
		&OAuthAuthorizationCode{},
		&CasbinRule{},
		&auditlog.OperationLog{},
	)
}