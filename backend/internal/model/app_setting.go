package model

import "time"

// AppSetting is a key/value row in app_settings. Today the only key in active
// use is "low_stock_threshold" (value = stringified int32); the table is
// generic so future shop-wide settings can land without a new migration.
type AppSetting struct {
	Key       string    `gorm:"primaryKey;column:key"`
	Value     string    `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null;column:updated_at"`
}

func (AppSetting) TableName() string { return "app_settings" }
