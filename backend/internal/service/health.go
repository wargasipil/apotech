package service

import (
	"context"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	healthifacev1 "github.com/apotech/backend/gen/health_iface/v1"
)

type Health struct {
	db *gorm.DB
}

func NewHealth(db *gorm.DB) *Health {
	return &Health{db: db}
}

func (h *Health) Ping(
	ctx context.Context,
	_ *connect.Request[healthifacev1.PingRequest],
) (*connect.Response[healthifacev1.PingResponse], error) {
	dbStatus := "ok"
	sqlDB, err := h.db.DB()
	if err != nil {
		dbStatus = "error: " + err.Error()
	} else if err := sqlDB.PingContext(ctx); err != nil {
		dbStatus = "error: " + err.Error()
	}

	return connect.NewResponse(&healthifacev1.PingResponse{
		Status: "ok",
		Db:     dbStatus,
	}), nil
}
