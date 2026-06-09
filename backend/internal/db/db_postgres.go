//go:build !sqlite

package db

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/apotech/backend/internal/config"
)

// openDialect returns the Postgres dialector for the default build. The
// `-tags sqlite` build replaces this file with db_sqlite.go.
func openDialect(cfg *config.Config) (gorm.Dialector, error) {
	if cfg.Database.IsSQLite() {
		return nil, fmt.Errorf("config requests sqlite driver but this binary was built without the sqlite tag; rebuild with `go build -tags sqlite`")
	}
	return postgres.Open(cfg.Database.DSN()), nil
}
