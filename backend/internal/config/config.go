package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Server struct {
	// Host is the interface to bind. "0.0.0.0" = all interfaces (LAN/Docker),
	// "127.0.0.1" = this machine only. Empty defaults to "0.0.0.0" (see Load).
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	// OpenBrowserOnStart triggers a default-browser open at the bound URL
	// right after the listener is ready. Off by default; the portable Windows
	// installer's config sets it true so the EXE doubles as a launcher.
	OpenBrowserOnStart bool `yaml:"open_browser_on_start"`
}

type Database struct {
	// Driver selects which backend powers the app. "postgres" (default) drives
	// the production / docker-compose deployment; "sqlite" drives the portable
	// single-PC Windows flavor. Two separate binaries are produced via Go build
	// tags — IsSQLite reports the configured intent.
	Driver string `yaml:"driver"`
	// Path is the SQLite file path. Ignored when Driver == "postgres". Empty
	// defaults to "./data/apotech.db" (CWD-relative).
	Path     string `yaml:"path"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
	SSLMode  string `yaml:"sslmode"`
	// AutoMigrate runs goose migrations on server boot. Pointer so an unset
	// value defaults to true (turnkey deploys); set `auto_migrate: false` to
	// run migrations explicitly via `cmd/migrate`. Read via ShouldAutoMigrate.
	AutoMigrate *bool `yaml:"auto_migrate"`
}

// ShouldAutoMigrate reports whether the server should run migrations on boot.
// Defaults to true when unset.
func (d Database) ShouldAutoMigrate() bool {
	return d.AutoMigrate == nil || *d.AutoMigrate
}

// IsSQLite reports whether the configured driver is SQLite. The Postgres build
// tag rejects this at boot (driver mismatch); the SQLite build tag requires it.
func (d Database) IsSQLite() bool {
	return d.Driver == "sqlite"
}

type Auth struct {
	JWTSecret       string        `yaml:"jwt_secret"`
	AccessTokenTTL  time.Duration `yaml:"access_token_ttl"`
	RefreshTokenTTL time.Duration `yaml:"refresh_token_ttl"`
}

type Bootstrap struct {
	OwnerEmail    string `yaml:"owner_email"`
	OwnerPassword string `yaml:"owner_password"`
}

type Printer struct {
	Enabled bool          `yaml:"enabled"`
	Address string        `yaml:"address"`        // host:port (raw TCP, typically port 9100)
	Width   int           `yaml:"width"`          // chars per line (32 for 58mm, 48 for 80mm)
	Timeout time.Duration `yaml:"timeout"`        // dial+write timeout
	Header  []string      `yaml:"header"`         // shop name/address lines printed on top
	Footer  []string      `yaml:"footer"`         // closing lines (e.g. "Thank you!")
	OpenDrawer bool       `yaml:"open_drawer"`    // send drawer-kick command after print
}

// Backup controls where BackupService writes per-timestamp backup directories.
// Empty Directory defaults to ./backups (CWD-relative, matches the legacy
// `make backup` behavior). Docker uses /var/lib/apotech/backups; Windows uses
// C:\ProgramData\Apotech\backups.
//
// PgToolsDir is where the in-app Create-backup feature caches the pg_dump
// binary it auto-downloads when none is found on PATH / bundled next to the
// apotech binary. Empty defaults to <UserCacheDir>/apotech/pgtools
// (Windows → %LOCALAPPDATA%\apotech\pgtools; Linux → ~/.cache/apotech/pgtools).
// Docker never triggers the auto-download (pg_dump is on PATH there), so this
// dir stays empty in containers.
type Backup struct {
	Directory  string `yaml:"directory"`
	PgToolsDir string `yaml:"pg_tools_dir"`
}

type Config struct {
	Server    Server    `yaml:"server"`
	Database  Database  `yaml:"database"`
	Auth      Auth      `yaml:"auth"`
	Bootstrap Bootstrap `yaml:"bootstrap"`
	Printer   Printer   `yaml:"printer"`
	Backup    Backup    `yaml:"backup"`
}

func (d Database) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = os.Getenv("APOTECH_CONFIG")
		if path == "" {
			path = "config.yaml"
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %s: %w", path, err)
	}
	defer f.Close()

	var c Config
	if err := yaml.NewDecoder(f).Decode(&c); err != nil {
		return nil, fmt.Errorf("decode config %s: %w", path, err)
	}
	applyEnvOverrides(&c)
	applyDefaults(&c)
	return &c, nil
}

// applyEnvOverrides lets the most security-sensitive fields be supplied via
// environment variables instead of the YAML file. This keeps secrets out of a
// baked Docker image (12-factor) while the YAML still drives everything else.
// An empty env var is treated as "not set" and leaves the YAML value intact.
func applyEnvOverrides(c *Config) {
	if v := os.Getenv("APOTECH_JWT_SECRET"); v != "" {
		c.Auth.JWTSecret = v
	}
	if v := os.Getenv("APOTECH_DB_DRIVER"); v != "" {
		c.Database.Driver = v
	}
	if v := os.Getenv("APOTECH_DB_PATH"); v != "" {
		c.Database.Path = v
	}
	if v := os.Getenv("APOTECH_DB_HOST"); v != "" {
		c.Database.Host = v
	}
	if v := os.Getenv("APOTECH_DB_PASSWORD"); v != "" {
		c.Database.Password = v
	}
	if v := os.Getenv("APOTECH_OWNER_EMAIL"); v != "" {
		c.Bootstrap.OwnerEmail = v
	}
	if v := os.Getenv("APOTECH_OWNER_PASSWORD"); v != "" {
		c.Bootstrap.OwnerPassword = v
	}
	if v := os.Getenv("APOTECH_BACKUP_DIR"); v != "" {
		c.Backup.Directory = v
	}
	if v := os.Getenv("APOTECH_PG_TOOLS_DIR"); v != "" {
		c.Backup.PgToolsDir = v
	}
}

// applyDefaults fills in safe fallbacks for fields that the packaged flavors
// rely on. Bind to all interfaces by default so a container is reachable from
// the host; single-PC installs override this to 127.0.0.1.
func applyDefaults(c *Config) {
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Database.Driver == "" {
		c.Database.Driver = "postgres"
	}
	if c.Database.IsSQLite() && c.Database.Path == "" {
		c.Database.Path = "./data/apotech.db"
	}
	if c.Backup.Directory == "" {
		c.Backup.Directory = "./backups"
	}
	if c.Backup.PgToolsDir == "" {
		// os.UserCacheDir picks the right per-user cache root on each OS
		// (Windows: %LOCALAPPDATA%; Linux: $XDG_CACHE_HOME or ~/.cache;
		// macOS: ~/Library/Caches). Falls back to the backup directory if
		// the OS doesn't expose a cache root.
		if base, err := os.UserCacheDir(); err == nil {
			c.Backup.PgToolsDir = base + string(os.PathSeparator) + "apotech" + string(os.PathSeparator) + "pgtools"
		} else {
			c.Backup.PgToolsDir = c.Backup.Directory + string(os.PathSeparator) + "_pgtools"
		}
	}
}

func MustLoad() *Config {
	c, err := Load("")
	if err != nil {
		panic(err)
	}
	return c
}
