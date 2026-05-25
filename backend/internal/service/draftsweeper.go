package service

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/apotech/backend/internal/model"
)

const (
	// How often the sweeper runs, and how long a DRAFT sale may sit idle before
	// it's considered abandoned. In-process, single-node posture (like the rate
	// limiter); not configurable for now.
	draftSweepInterval = time.Hour
	draftMaxIdle       = 24 * time.Hour
)

// SweepStaleDrafts hard-deletes DRAFT sales (and their items) whose updated_at is
// older than maxIdle, returning the number of sales deleted. Exported so tests
// can drive it directly without the goroutine. Safe to delete: a DRAFT has no
// stock_movements and no sale_no, so nothing references it (see DiscardSale).
func SweepStaleDrafts(ctx context.Context, db *gorm.DB, maxIdle time.Duration) (int64, error) {
	cutoff := time.Now().Add(-maxIdle)
	var deleted int64
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ids []string
		if err := tx.Model(&model.Sale{}).
			Where("status = ? AND updated_at < ?", saleStatusDraft, cutoff).
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		if err := tx.Where("sale_id IN ?", ids).Delete(&model.SaleItem{}).Error; err != nil {
			return err
		}
		res := tx.Where("id IN ?", ids).Delete(&model.Sale{})
		if res.Error != nil {
			return res.Error
		}
		deleted = res.RowsAffected
		return nil
	})
	return deleted, err
}

// StartDraftSweeper launches a background goroutine that periodically deletes
// abandoned DRAFT carts (idle > draftMaxIdle) the POS client missed (crashes,
// lost sessions). Runs for the process lifetime; no graceful shutdown.
func StartDraftSweeper(db *gorm.DB) {
	go func() {
		ticker := time.NewTicker(draftSweepInterval)
		defer ticker.Stop()
		for range ticker.C {
			n, err := SweepStaleDrafts(context.Background(), db, draftMaxIdle)
			if err != nil {
				slog.Error("draft sweeper failed", "error", err)
				continue
			}
			if n > 0 {
				slog.Info("draft sweeper deleted abandoned carts", "count", n)
			}
		}
	}()
}
