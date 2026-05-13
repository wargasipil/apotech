package db

import (
	"context"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/apotech/backend/internal/config"
)

func Open(cfg *config.Config) (*gorm.DB, error) {
	gormDB, err := gorm.Open(postgres.Open(cfg.Database.DSN()), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, err
	}
	if err := sqlDB.PingContext(context.Background()); err != nil {
		return nil, err
	}
	return gormDB, nil
}

func MustOpen(cfg *config.Config) *gorm.DB {
	db, err := Open(cfg)
	if err != nil {
		panic(err)
	}
	return db
}
