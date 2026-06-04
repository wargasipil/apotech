package db

import (
	"context"
	"reflect"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/apotech/backend/internal/config"
)

func Open(cfg *config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector
	if cfg.Database.IsSQLite() {
		dialector = sqlite.Open(cfg.Database.SQLiteDSN())
	} else {
		dialector = postgres.Open(cfg.Database.DSN())
	}

	gormDB, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := registerUUIDCallback(gormDB); err != nil {
		return nil, err
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, err
	}
	if cfg.Database.IsSQLite() {
		// SQLite serializes writers anyway (BEGIN IMMEDIATE via _txlock); a
		// single connection makes that bulletproof — no SQLITE_BUSY, no lost
		// races on the insert-only stock ledger. Fine for the single-PC target.
		sqlDB.SetMaxOpenConns(1)
	}
	if err := sqlDB.PingContext(context.Background()); err != nil {
		return nil, err
	}
	return gormDB, nil
}

// registerUUIDCallback installs a global BeforeCreate hook that fills an empty
// string primary key named "ID" with a fresh UUID. This replaces Postgres's
// DB-side gen_random_uuid() default with Go-side generation, which is required
// on SQLite (no such function) and is harmless on Postgres (the value is then
// supplied, so the DEFAULT never fires). It deliberately skips:
//   - integer autoincrement PKs (audit_log.id),
//   - composite / FK primary keys (user_warehouses), and
//   - non-"ID" string PKs (app_settings.key, *_no_counters.year)
//
// by only acting on a single primary field whose Go name is exactly "ID",
// whose kind is string, and which is currently empty. Runs for slice creates
// and ON CONFLICT inserts too (GORM invokes BeforeCreate per row).
func registerUUIDCallback(db *gorm.DB) error {
	return db.Callback().Create().Before("gorm:create").Register("apotech:set_uuid", func(tx *gorm.DB) {
		st := tx.Statement
		if st == nil || st.Schema == nil {
			return
		}
		field := st.Schema.PrioritizedPrimaryField
		if field == nil || field.Name != "ID" || field.FieldType.Kind() != reflect.String {
			return
		}
		setIfEmpty := func(rv reflect.Value) {
			if v, zero := field.ValueOf(st.Context, rv); zero || v == "" {
				_ = field.Set(st.Context, rv, uuid.NewString())
			}
		}
		switch st.ReflectValue.Kind() {
		case reflect.Slice, reflect.Array:
			for i := 0; i < st.ReflectValue.Len(); i++ {
				setIfEmpty(st.ReflectValue.Index(i))
			}
		case reflect.Struct:
			setIfEmpty(st.ReflectValue)
		}
	})
}

func MustOpen(cfg *config.Config) *gorm.DB {
	db, err := Open(cfg)
	if err != nil {
		panic(err)
	}
	return db
}
