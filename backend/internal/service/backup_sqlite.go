//go:build sqlite

package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"connectrpc.com/connect"
)

// dumpFileName is the per-build-tag output filename inside backup_<ts>/.
// SQLite flavor writes a copy of the database file via VACUUM INTO.
const dumpFileName = "database.db"

// performDump runs `VACUUM INTO 'dir/database.db'` (atomic, consistent SQLite
// snapshot, no external binary) and returns the file size.
func (s *Backups) performDump(ctx context.Context, dir string) (int64, error) {
	dumpPath := filepath.Join(dir, dumpFileName)
	// VACUUM INTO requires the destination not to exist yet — the directory
	// was just mkdir'd so the file is guaranteed absent, but the explicit
	// remove guards re-runs in odd states.
	_ = os.Remove(dumpPath)
	if err := s.db.WithContext(ctx).Exec("VACUUM INTO ?", dumpPath).Error; err != nil {
		return 0, connect.NewError(connect.CodeInternal, fmt.Errorf("vacuum into: %w", err))
	}
	info, err := os.Stat(dumpPath)
	if err != nil {
		return 0, connect.NewError(connect.CodeInternal, fmt.Errorf("stat dump: %w", err))
	}
	return info.Size(), nil
}

// readDBVersion returns the SQLite library version for the manifest.
func (s *Backups) readDBVersion(ctx context.Context) string {
	var v string
	if err := s.db.WithContext(ctx).Raw(`SELECT sqlite_version()`).Scan(&v).Error; err != nil {
		return ""
	}
	return v
}
