//go:build !sqlite

package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"connectrpc.com/connect"
)

// dumpFileName is the per-build-tag output filename inside backup_<ts>/.
// Postgres flavor writes a compressed pg_dump SQL stream.
const dumpFileName = "database.sql.gz"

// performDump runs pg_dump into dir/database.sql.gz and returns the file size.
// Resolves pg_dump via PATH → bundled-next-to-binary → cached download →
// (Windows + autoFetch) auto-download EDB binaries → friendly error.
func (s *Backups) performDump(ctx context.Context, dir string) (int64, error) {
	pgDumpPath, err := resolvePgDump(ctx, s.pgToolsDir, s.autoFetchPgDump)
	if err != nil {
		return 0, connect.NewError(connect.CodeFailedPrecondition, err)
	}

	dumpPath := filepath.Join(dir, dumpFileName)
	db := s.cfg.Database
	cmd := exec.CommandContext(ctx, pgDumpPath,
		"--host="+db.Host,
		"--port="+strconv.Itoa(db.Port),
		"--username="+db.User,
		"--dbname="+db.Name,
		"--no-password",
		"--clean",
		"--if-exists",
		"--compress=6",
		"--file="+dumpPath,
	)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+db.Password)
	stderr := &strings.Builder{}
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return 0, connect.NewError(connect.CodeInternal, fmt.Errorf("pg_dump: %s", msg))
	}
	info, err := os.Stat(dumpPath)
	if err != nil {
		return 0, connect.NewError(connect.CodeInternal, fmt.Errorf("stat dump: %w", err))
	}
	return info.Size(), nil
}

// readDBVersion returns the Postgres server version string for the manifest.
func (s *Backups) readDBVersion(ctx context.Context) string {
	var v string
	if err := s.db.WithContext(ctx).Raw(`SELECT version()`).Scan(&v).Error; err != nil {
		return ""
	}
	return v
}
