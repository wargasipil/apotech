package service

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	backupifacev1 "github.com/apotech/backend/gen/backup_iface/v1"
	"github.com/apotech/backend/internal/config"
)

// Backups writes per-timestamp pg_dump archives into a configured directory:
//
//	<cfg.Backup.Directory>/backup_<YYYY-mm-dd_HHMMSS>/
//	  database.sql.gz   (pg_dump --compress=6 output)
//	  manifest.txt      (created_at + db/schema versions + size_bytes)
//
// All RPCs are OWNER-only (enforced at the proto layer via allowed_roles).
//
// Restore is intentionally NOT exposed via RPC — it needs a maintenance-mode
// UX. CLI restore is documented in DEPLOYMENT.md.
type Backups struct {
	db        *gorm.DB
	cfg       *config.Config
	directory string // absolute or CWD-relative; created lazily on Create.
	// pgToolsDir is where resolvePgDump caches an auto-downloaded pg_dump
	// (Windows dev fallback). Empty disables caching/auto-download.
	pgToolsDir string
	// autoFetchPgDump enables the EDB binaries auto-download on Windows when
	// no pg_dump is found on PATH or bundled next to the apotech binary.
	// Production (NewBackups) is true; tests (NewBackupsWithDir) are false so
	// a missing pg_dump skips cleanly without downloading 75 MB.
	autoFetchPgDump bool
}

// NewBackups uses cfg.Backup.Directory + cfg.Backup.PgToolsDir and enables
// auto-fetch. Test code uses NewBackupsWithDir.
func NewBackups(db *gorm.DB, cfg *config.Config) *Backups {
	return &Backups{
		db:              db,
		cfg:             cfg,
		directory:       cfg.Backup.Directory,
		pgToolsDir:      cfg.Backup.PgToolsDir,
		autoFetchPgDump: true,
	}
}

// NewBackupsWithDir is the test constructor — point it at t.TempDir(). Tests
// keep autoFetchPgDump false so a missing pg_dump fails fast (via the existing
// LookPath skip guard) instead of triggering a 75 MB download.
func NewBackupsWithDir(db *gorm.DB, cfg *config.Config, directory string) *Backups {
	return &Backups{
		db:              db,
		cfg:             cfg,
		directory:       directory,
		pgToolsDir:      filepath.Join(directory, "_pgtools"),
		autoFetchPgDump: false,
	}
}

// backupNameRe matches a per-timestamp directory name. The same regex
// validates Delete's `name` arg, refusing `..`, slashes, anything else.
var backupNameRe = regexp.MustCompile(`^backup_\d{4}-\d{2}-\d{2}_\d{6}$`)

const (
	dumpFileName     = "database.sql.gz"
	manifestFileName = "manifest.txt"
)

// CreateBackup runs pg_dump (compressed) into a fresh backup_<ts>/ directory.
// On any failure the partial directory is removed so the listing never shows
// a half-baked backup.
func (s *Backups) CreateBackup(
	ctx context.Context,
	_ *connect.Request[backupifacev1.CreateBackupRequest],
) (*connect.Response[backupifacev1.CreateBackupResponse], error) {
	if err := s.ensureDir(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	now := time.Now()
	name := "backup_" + now.Format("2006-01-02_150405")
	dir := filepath.Join(s.directory, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("mkdir backup: %w", err))
	}
	// Belt + suspenders: if anything below fails, drop the partial dir.
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(dir)
		}
	}()

	// Resolve pg_dump: PATH → bundled-next-to-apotech-binary → cached download →
	// (Windows + autoFetch) auto-download EDB binaries → friendly error.
	pgDumpPath, err := resolvePgDump(ctx, s.pgToolsDir, s.autoFetchPgDump)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
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
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("pg_dump: %s", msg))
	}

	info, err := os.Stat(dumpPath)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("stat dump: %w", err))
	}
	size := info.Size()

	schemaVersion := s.readSchemaVersion(ctx)
	dbVersion := s.readDBVersion(ctx)

	if err := writeManifest(filepath.Join(dir, manifestFileName), manifest{
		CreatedAt:     now.Unix(),
		AppVersion:    appVersion,
		DBVersion:     dbVersion,
		SchemaVersion: schemaVersion,
		SizeBytes:     size,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("write manifest: %w", err))
	}

	committed = true
	return connect.NewResponse(&backupifacev1.CreateBackupResponse{
		Backup: &backupifacev1.Backup{
			Name:          name,
			CreatedAt:     now.Unix(),
			SizeBytes:     size,
			SchemaVersion: schemaVersion,
		},
	}), nil
}

// ListBackups returns every backup_<timestamp>/ directory under the configured
// backup root, newest first. Non-matching entries (files, foreign dirs) are
// silently ignored.
func (s *Backups) ListBackups(
	_ context.Context,
	_ *connect.Request[backupifacev1.ListBackupsRequest],
) (*connect.Response[backupifacev1.ListBackupsResponse], error) {
	out := &backupifacev1.ListBackupsResponse{}
	entries, err := os.ReadDir(s.directory)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// No backups have ever been taken — empty list, not an error.
			return connect.NewResponse(out), nil
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, e := range entries {
		if !e.IsDir() || !backupNameRe.MatchString(e.Name()) {
			continue
		}
		dir := filepath.Join(s.directory, e.Name())
		info, err := os.Stat(filepath.Join(dir, dumpFileName))
		var size int64
		if err == nil {
			size = info.Size()
		}
		// Parse the timestamp out of the name; fall back to dir mtime so a
		// listing never silently drops a real backup.
		var created int64
		if t, perr := time.ParseInLocation("2006-01-02_150405",
			strings.TrimPrefix(e.Name(), "backup_"), time.Local); perr == nil {
			created = t.Unix()
		} else if di, derr := e.Info(); derr == nil {
			created = di.ModTime().Unix()
		}
		schemaVersion := readManifestSchemaVersion(filepath.Join(dir, manifestFileName))
		out.Backups = append(out.Backups, &backupifacev1.Backup{
			Name:          e.Name(),
			CreatedAt:     created,
			SizeBytes:     size,
			SchemaVersion: schemaVersion,
		})
	}
	sort.Slice(out.Backups, func(i, j int) bool {
		return out.Backups[i].CreatedAt > out.Backups[j].CreatedAt
	})
	return connect.NewResponse(out), nil
}

// DeleteBackup removes one backup_<timestamp>/ directory. The name is
// validated against backupNameRe before any filesystem op, so `..` /
// absolute paths / slashes / unrelated dirs cannot be deleted via this RPC.
func (s *Backups) DeleteBackup(
	_ context.Context,
	req *connect.Request[backupifacev1.DeleteBackupRequest],
) (*connect.Response[backupifacev1.DeleteBackupResponse], error) {
	name := req.Msg.Name
	if !backupNameRe.MatchString(name) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("invalid backup name"))
	}
	dir := filepath.Join(s.directory, name)
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("backup not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := os.RemoveAll(dir); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&backupifacev1.DeleteBackupResponse{}), nil
}

func (s *Backups) ensureDir() error {
	return os.MkdirAll(s.directory, 0o755)
}

// appVersion is a compile-time constant for the manifest. Wire a real value
// here later (ldflags); "dev" is fine for now.
const appVersion = "dev"

func (s *Backups) readSchemaVersion(ctx context.Context) int32 {
	var v int64
	// goose writes (version_id, is_applied) rows; the max applied id is the
	// current schema version. Don't fail backup if the table is missing.
	err := s.db.WithContext(ctx).Raw(
		`SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version WHERE is_applied = true`,
	).Scan(&v).Error
	if err != nil {
		return 0
	}
	return int32(v)
}

func (s *Backups) readDBVersion(ctx context.Context) string {
	var v string
	if err := s.db.WithContext(ctx).Raw(`SELECT version()`).Scan(&v).Error; err != nil {
		return ""
	}
	return v
}

// manifest is the on-disk shape of manifest.txt. Plain key=value lines so
// `cat manifest.txt` works without any tooling.
type manifest struct {
	CreatedAt     int64
	AppVersion    string
	DBVersion     string
	SchemaVersion int32
	SizeBytes     int64
}

func writeManifest(path string, m manifest) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	fmt.Fprintf(w, "created_at=%d\n", m.CreatedAt)
	fmt.Fprintf(w, "created_at_iso=%s\n", time.Unix(m.CreatedAt, 0).Format(time.RFC3339))
	fmt.Fprintf(w, "app_version=%s\n", m.AppVersion)
	fmt.Fprintf(w, "db_version=%s\n", m.DBVersion)
	fmt.Fprintf(w, "schema_version=%d\n", m.SchemaVersion)
	fmt.Fprintf(w, "size_bytes=%d\n", m.SizeBytes)
	return w.Flush()
}

// readManifestSchemaVersion best-effort parses schema_version out of an
// existing manifest. Returns 0 if the file is missing or unparseable — the
// listing still shows the row.
func readManifestSchemaVersion(path string) int32 {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "schema_version=") {
			continue
		}
		n, perr := strconv.ParseInt(strings.TrimPrefix(line, "schema_version="), 10, 32)
		if perr != nil {
			return 0
		}
		return int32(n)
	}
	return 0
}
