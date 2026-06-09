package db

import (
	"context"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"github.com/apotech/backend/internal/config"
)

// Open opens the GORM DB using the dialect selected at compile time by build
// tag: db_postgres.go (default, no tag) or db_sqlite.go (`-tags sqlite`).
// openDialect is defined in exactly one of those files.
func Open(cfg *config.Config) (*gorm.DB, error) {
	dialect, err := openDialect(cfg)
	if err != nil {
		return nil, err
	}
	gormDB, err := gorm.Open(dialect, &gorm.Config{})
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
	if err := registerUUIDPKCallback(gormDB); err != nil {
		return nil, err
	}
	return gormDB, nil
}

// registerUUIDPKCallback installs a Before-Create hook that fills any empty
// string primary key whose GORM column type is "uuid" with a freshly
// generated UUID. This replaces the schema-level `default:gen_random_uuid()`
// — Postgres still has that function, but SQLite doesn't, so we generate the
// UUID in Go and bind it as a parameter. Driver-agnostic; both flavors share
// the same Go-side ID assignment.
func registerUUIDPKCallback(db *gorm.DB) error {
	return db.Callback().Create().Before("gorm:before_create").Register(
		"apotech:uuid_pk",
		func(tx *gorm.DB) {
			stmt := tx.Statement
			if stmt == nil || stmt.Schema == nil {
				return
			}
			rv := stmt.ReflectValue
			switch rv.Kind() {
			case reflect.Slice, reflect.Array:
				for i := 0; i < rv.Len(); i++ {
					fillUUIDFields(stmt.Schema, rv.Index(i))
				}
			case reflect.Struct:
				fillUUIDFields(stmt.Schema, rv)
			}
		},
	)
}

func fillUUIDFields(schema *schema.Schema, rv reflect.Value) {
	for _, field := range schema.PrimaryFields {
		if !isUUIDStringField(field) {
			continue
		}
		// Walk pointers down to the addressable struct.
		val := rv
		for val.Kind() == reflect.Pointer {
			if val.IsNil() {
				return
			}
			val = val.Elem()
		}
		if val.Kind() != reflect.Struct {
			return
		}
		fv := val.FieldByIndex(field.StructField.Index)
		if !fv.IsValid() || fv.Kind() != reflect.String {
			continue
		}
		if fv.String() != "" {
			continue
		}
		if !fv.CanSet() {
			continue
		}
		fv.SetString(uuid.NewString())
	}
}

func isUUIDStringField(field *schema.Field) bool {
	if field == nil || field.FieldType.Kind() != reflect.String {
		return false
	}
	dataType := strings.ToLower(string(field.DataType))
	if dataType == "uuid" {
		return true
	}
	if t, ok := field.TagSettings["TYPE"]; ok && strings.EqualFold(t, "uuid") {
		return true
	}
	return false
}

func MustOpen(cfg *config.Config) *gorm.DB {
	db, err := Open(cfg)
	if err != nil {
		panic(err)
	}
	return db
}
