package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	backupifacev1 "github.com/apotech/backend/gen/backup_iface/v1"
)

// TestBackupService_SQLite proves the SQLite backup path: CreateBackup writes a
// VACUUM INTO snapshot (backup_<ts>/database.db) + manifest.txt with no pg_dump,
// ListBackups surfaces it with the right size, and DeleteBackup removes it.
// Runs only on the SQLite backend (APOTECH_DB_DRIVER=sqlite).
func TestBackupService_SQLite(t *testing.T) {
	env := SetupEnv(t)
	if env.DB.Dialector.Name() != "sqlite" {
		t.Skip("not the SQLite backend — covered by TestBackupService on Postgres")
	}
	ctx := context.Background()

	ls0, err := env.Backups.ListBackups(ctx, authReq(env, t, &backupifacev1.ListBackupsRequest{}))
	require.NoError(t, err)
	require.Empty(t, ls0.Msg.Backups, "fresh BackupDir should be empty")

	c1, err := env.Backups.CreateBackup(ctx, authReq(env, t, &backupifacev1.CreateBackupRequest{}))
	require.NoError(t, err)
	require.Regexp(t, `^backup_\d{4}-\d{2}-\d{2}_\d{6}$`, c1.Msg.Backup.Name)
	require.Greater(t, c1.Msg.Backup.SizeBytes, int64(0))

	dir1 := filepath.Join(env.BackupDir, c1.Msg.Backup.Name)
	dumpInfo, err := os.Stat(filepath.Join(dir1, "database.db"))
	require.NoError(t, err, "database.db must exist after Create (VACUUM INTO)")
	require.Greater(t, dumpInfo.Size(), int64(0))

	// The snapshot is itself a valid SQLite database (magic header). Read +
	// close before DeleteBackup — Windows can't remove a still-open file.
	head, err := os.ReadFile(filepath.Join(dir1, "database.db"))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(head), "SQLite format 3"), "dump should be a real SQLite file")

	// Manifest records schema_version (>0) and a SQLite db_version.
	man := readManifest(t, filepath.Join(dir1, "manifest.txt"))
	require.Equal(t, "30", man["schema_version"], "schema_version should reflect the migrated version")
	require.True(t, strings.HasPrefix(man["db_version"], "SQLite"), "db_version should be sqlite_version(), got %q", man["db_version"])

	ls1, err := env.Backups.ListBackups(ctx, authReq(env, t, &backupifacev1.ListBackupsRequest{}))
	require.NoError(t, err)
	require.Len(t, ls1.Msg.Backups, 1)
	require.Equal(t, c1.Msg.Backup.Name, ls1.Msg.Backups[0].Name)
	require.Greater(t, ls1.Msg.Backups[0].SizeBytes, int64(0))

	_, err = env.Backups.DeleteBackup(ctx, authReq(env, t, &backupifacev1.DeleteBackupRequest{Name: c1.Msg.Backup.Name}))
	require.NoError(t, err)
	ls2, err := env.Backups.ListBackups(ctx, authReq(env, t, &backupifacev1.ListBackupsRequest{}))
	require.NoError(t, err)
	require.Empty(t, ls2.Msg.Backups)
}
