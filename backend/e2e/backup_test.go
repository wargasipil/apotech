//go:build !sqlite

package e2e

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	backupifacev1 "github.com/apotech/backend/gen/backup_iface/v1"
	"github.com/apotech/backend/internal/service"
)

// TestBackupService proves the per-timestamp directory layout end-to-end:
//   - CreateBackup writes <tempdir>/backup_<ts>/{database.sql.gz, manifest.txt}
//     with a non-empty dump, and ListBackups surfaces it.
//   - A second CreateBackup leaves two backups (newest first), and DeleteBackup
//     removes only the targeted one.
//   - DeleteBackup with a path-traversal name is rejected InvalidArgument.
//
// Skipped (not failed) when pg_dump isn't on PATH so a developer without it
// installed isn't blocked. The Dockerfile installs postgresql-client, the
// Windows installer bundles pg_dump.exe.
func TestBackupService(t *testing.T) {
	if _, err := exec.LookPath("pg_dump"); err != nil {
		t.Skipf("pg_dump not on PATH (%v) — skipping; install postgresql-client to run", err)
	}

	env := SetupEnv(t)
	ctx := context.Background()

	// 1. Fresh env → ListBackups returns empty.
	ls0, err := env.Backups.ListBackups(ctx, authReq(env, t, &backupifacev1.ListBackupsRequest{}))
	require.NoError(t, err)
	require.Empty(t, ls0.Msg.Backups, "fresh BackupDir should be empty")

	// 2. CreateBackup writes a per-timestamp dir + a non-empty gzip dump.
	c1, err := env.Backups.CreateBackup(ctx, authReq(env, t, &backupifacev1.CreateBackupRequest{}))
	require.NoError(t, err)
	require.NotEmpty(t, c1.Msg.Backup.Name)
	require.Regexp(t, `^backup_\d{4}-\d{2}-\d{2}_\d{6}$`, c1.Msg.Backup.Name)
	require.Greater(t, c1.Msg.Backup.SizeBytes, int64(0))

	dir1 := filepath.Join(env.BackupDir, c1.Msg.Backup.Name)
	dumpPath := filepath.Join(dir1, "database.sql.gz")
	dumpInfo, err := os.Stat(dumpPath)
	require.NoError(t, err, "database.sql.gz must exist after Create")
	require.Greater(t, dumpInfo.Size(), int64(0))
	manifestPath := filepath.Join(dir1, "manifest.txt")
	manifestInfo, err := os.Stat(manifestPath)
	require.NoError(t, err, "manifest.txt must exist after Create")
	require.Greater(t, manifestInfo.Size(), int64(0))

	// 3. ListBackups now returns the row.
	ls1, err := env.Backups.ListBackups(ctx, authReq(env, t, &backupifacev1.ListBackupsRequest{}))
	require.NoError(t, err)
	require.Len(t, ls1.Msg.Backups, 1)
	require.Equal(t, c1.Msg.Backup.Name, ls1.Msg.Backups[0].Name)

	// 4. Second Create → two backups, newest first.
	c2, err := env.Backups.CreateBackup(ctx, authReq(env, t, &backupifacev1.CreateBackupRequest{}))
	require.NoError(t, err)
	require.NotEqual(t, c1.Msg.Backup.Name, c2.Msg.Backup.Name,
		"second backup should land in a distinct timestamp dir")
	ls2, err := env.Backups.ListBackups(ctx, authReq(env, t, &backupifacev1.ListBackupsRequest{}))
	require.NoError(t, err)
	require.Len(t, ls2.Msg.Backups, 2)
	require.GreaterOrEqual(t, ls2.Msg.Backups[0].CreatedAt, ls2.Msg.Backups[1].CreatedAt,
		"list is newest first")

	// 5. DeleteBackup removes the targeted dir; the other survives.
	_, err = env.Backups.DeleteBackup(ctx, authReq(env, t,
		&backupifacev1.DeleteBackupRequest{Name: c1.Msg.Backup.Name}))
	require.NoError(t, err)
	_, err = os.Stat(dir1)
	require.True(t, os.IsNotExist(err), "deleted dir should be gone")
	_, err = os.Stat(filepath.Join(env.BackupDir, c2.Msg.Backup.Name))
	require.NoError(t, err, "other backup must still exist")

	// 6. Path-traversal name is rejected without touching the filesystem.
	_, err = env.Backups.DeleteBackup(ctx, authReq(env, t,
		&backupifacev1.DeleteBackupRequest{Name: "../etc"}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	// 7. Deleting a non-existent (but well-formed) name is NotFound.
	_, err = env.Backups.DeleteBackup(ctx, authReq(env, t,
		&backupifacev1.DeleteBackupRequest{Name: "backup_1999-01-01_000000"}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// TestBackup_ManifestContents proves the writeManifest → readManifestSchemaVersion
// round-trip: every key the service writes is parseable, schema_version matches
// the ListBackups response, size_bytes matches the on-disk dump, and the
// app/db versions look right.
func TestBackup_ManifestContents(t *testing.T) {
	if _, err := exec.LookPath("pg_dump"); err != nil {
		t.Skipf("pg_dump not on PATH (%v) — skipping", err)
	}

	env := SetupEnv(t)
	ctx := context.Background()

	c, err := env.Backups.CreateBackup(ctx, authReq(env, t, &backupifacev1.CreateBackupRequest{}))
	require.NoError(t, err)
	name := c.Msg.Backup.Name

	// Parse manifest.txt into a key=value map.
	manifest := readManifest(t, filepath.Join(env.BackupDir, name, "manifest.txt"))
	for _, key := range []string{"created_at", "created_at_iso", "app_version", "db_version", "schema_version", "size_bytes"} {
		require.Contains(t, manifest, key, "manifest must carry %q", key)
	}

	// schema_version in the file matches the ListBackups response.
	mSchema, err := strconv.ParseInt(manifest["schema_version"], 10, 32)
	require.NoError(t, err)
	require.Equal(t, int32(mSchema), c.Msg.Backup.SchemaVersion,
		"manifest schema_version must match the Backup proto")

	// size_bytes matches the on-disk dump size (and is non-zero).
	mSize, err := strconv.ParseInt(manifest["size_bytes"], 10, 64)
	require.NoError(t, err)
	require.Greater(t, mSize, int64(0))
	info, err := os.Stat(filepath.Join(env.BackupDir, name, "database.sql.gz"))
	require.NoError(t, err)
	require.Equal(t, info.Size(), mSize, "manifest size_bytes must match the file")

	// Sanity on the version strings.
	require.Equal(t, "dev", manifest["app_version"],
		"app_version is a compile-time const")
	require.True(t, strings.HasPrefix(manifest["db_version"], "PostgreSQL"),
		"db_version must come from SELECT version(); got %q", manifest["db_version"])
}

// TestBackup_ListIgnoresStrayEntries proves the regex filter in ListBackups
// silently skips files, foreign directories, and dirs whose name nearly looks
// like a backup but doesn't match the timestamp regex.
func TestBackup_ListIgnoresStrayEntries(t *testing.T) {
	if _, err := exec.LookPath("pg_dump"); err != nil {
		t.Skipf("pg_dump not on PATH (%v) — skipping", err)
	}

	env := SetupEnv(t)
	ctx := context.Background()

	// One legitimate backup.
	c, err := env.Backups.CreateBackup(ctx, authReq(env, t, &backupifacev1.CreateBackupRequest{}))
	require.NoError(t, err)
	legitName := c.Msg.Backup.Name

	// Stray entries the listing must ignore.
	require.NoError(t, os.WriteFile(filepath.Join(env.BackupDir, "random.txt"), []byte("hello"), 0o644))
	bogusDir := filepath.Join(env.BackupDir, "not_a_backup")
	require.NoError(t, os.Mkdir(bogusDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bogusDir, "stuff"), []byte("nope"), 0o644))
	// Right prefix, wrong digit count (5 instead of 6 in the time half).
	bogusBackup := filepath.Join(env.BackupDir, "backup_2026-01-01_12345")
	require.NoError(t, os.Mkdir(bogusBackup, 0o755))

	ls, err := env.Backups.ListBackups(ctx, authReq(env, t, &backupifacev1.ListBackupsRequest{}))
	require.NoError(t, err)
	require.Len(t, ls.Msg.Backups, 1, "stray entries must not appear in the listing")
	require.Equal(t, legitName, ls.Msg.Backups[0].Name)
}

// TestBackup_CleanupOnPgDumpFailure proves the defer block in CreateBackup
// removes a partial backup_<ts>/ directory when pg_dump exits non-zero. The
// failure is triggered with a bad-password clone of cfg.Database so pg_dump
// runs (proving the exec path, not the missing-binary path) but can't auth.
// Calls the Go method directly — bypasses Connect/auth so the test stays
// focused on the cleanup behavior.
func TestBackup_CleanupOnPgDumpFailure(t *testing.T) {
	if _, err := exec.LookPath("pg_dump"); err != nil {
		t.Skipf("pg_dump not on PATH (%v) — skipping", err)
	}

	env := SetupEnv(t)
	ctx := context.Background()

	// Clone the loaded config and break the DB password so pg_dump fails to
	// authenticate. Connection itself is still attempted (PG must be running),
	// which is what triggers the non-zero exit + the cleanup defer.
	badCfg := *env.Cfg
	badCfg.Database.Password = "definitely-wrong-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	tempDir := t.TempDir()
	s := service.NewBackupsWithDir(env.DB, &badCfg, tempDir)

	_, err := s.CreateBackup(ctx, connect.NewRequest(&backupifacev1.CreateBackupRequest{}))
	require.Error(t, err, "CreateBackup must fail when pg_dump can't authenticate")
	require.Contains(t, err.Error(), "pg_dump", "error must surface pg_dump stderr")

	// The partial backup_<ts>/ dir must have been removed by the defer in
	// CreateBackup — the temp dir holds no backup-shaped entries.
	entries, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	for _, e := range entries {
		require.False(t, strings.HasPrefix(e.Name(), "backup_"),
			"partial backup dir survived a pg_dump failure: %s", e.Name())
	}
}

// readManifest parses the simple key=value manifest.txt format that
// service.writeManifest emits. Lines without an `=` are skipped.
func readManifest(t *testing.T, path string) map[string]string {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}
		out[line[:idx]] = line[idx+1:]
	}
	require.NoError(t, sc.Err())
	return out
}
