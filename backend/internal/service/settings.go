package service

import (
	"context"
	"errors"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	settingsifacev1 "github.com/apotech/backend/gen/settings_iface/v1"
	"github.com/apotech/backend/internal/model"
)

const (
	settingKeyLowStockThreshold = "low_stock_threshold"
	defaultLowStockThreshold    = int32(10)
)

type Settings struct {
	db *gorm.DB
}

func NewSettings(db *gorm.DB) *Settings { return &Settings{db: db} }

func (s *Settings) GetSettings(
	ctx context.Context,
	_ *connect.Request[settingsifacev1.GetSettingsRequest],
) (*connect.Response[settingsifacev1.GetSettingsResponse], error) {
	threshold, err := getLowStockThreshold(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&settingsifacev1.GetSettingsResponse{
		Settings: &settingsifacev1.Settings{LowStockThreshold: threshold},
	}), nil
}

func (s *Settings) UpdateSettings(
	ctx context.Context,
	req *connect.Request[settingsifacev1.UpdateSettingsRequest],
) (*connect.Response[settingsifacev1.UpdateSettingsResponse], error) {
	if req.Msg.LowStockThreshold < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("low_stock_threshold must be >= 0"))
	}
	row := model.AppSetting{
		Key:       settingKeyLowStockThreshold,
		Value:     strconv.FormatInt(int64(req.Msg.LowStockThreshold), 10),
		UpdatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
		}).Create(&row).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&settingsifacev1.UpdateSettingsResponse{
		Settings: &settingsifacev1.Settings{LowStockThreshold: req.Msg.LowStockThreshold},
	}), nil
}

// getLowStockThreshold reads the current low-stock threshold from app_settings,
// returning the default (10) when no row exists or the stored value is invalid.
// Shared with MedicineService.ListLowStock.
func getLowStockThreshold(ctx context.Context, db *gorm.DB) (int32, error) {
	var row model.AppSetting
	err := db.WithContext(ctx).Where("key = ?", settingKeyLowStockThreshold).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return defaultLowStockThreshold, nil
	}
	if err != nil {
		return 0, err
	}
	n, perr := strconv.ParseInt(row.Value, 10, 32)
	if perr != nil {
		return defaultLowStockThreshold, nil
	}
	return int32(n), nil
}
