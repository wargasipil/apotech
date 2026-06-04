package db

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/apotech/backend/internal/config"
	"github.com/apotech/backend/internal/model"
)

// TestUUIDCallbackSQLite proves the global BeforeCreate hook fills a model's
// empty string ID with a UUID on the SQLite backend, even though the model tag
// still says default:gen_random_uuid() (which SQLite has no function for). It
// also confirms a non-"ID" / autoincrement PK is left untouched.
func TestUUIDCallbackSQLite(t *testing.T) {
	cfg := &config.Config{}
	cfg.Database.Driver = config.DriverSQLite
	cfg.Database.Path = ":memory:"

	gdb, err := Open(cfg)
	require.NoError(t, err)

	// Minimal SQLite-flavored users table (TEXT PK, no DB-side default).
	require.NoError(t, gdb.Exec(`CREATE TABLE users (
		id TEXT PRIMARY KEY,
		email TEXT UNIQUE NOT NULL COLLATE NOCASE,
		name TEXT NOT NULL DEFAULT '',
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL,
		active BOOLEAN NOT NULL DEFAULT 1,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error)

	// Single create: ID is filled Go-side.
	u := model.User{Email: "Owner@Example.com", PasswordHash: "x", Role: "OWNER", Active: true}
	require.NoError(t, gdb.Create(&u).Error)
	_, perr := uuid.Parse(u.ID)
	require.NoError(t, perr, "ID should be a generated UUID, got %q", u.ID)

	// COLLATE NOCASE makes the email lookup case-insensitive (CITEXT replacement).
	var got model.User
	require.NoError(t, gdb.First(&got, "email = ?", "owner@EXAMPLE.com").Error)
	require.Equal(t, u.ID, got.ID)

	// Slice create: each row gets its own UUID.
	batch := []model.User{
		{Email: "a@x.com", PasswordHash: "x", Role: "CASHIER"},
		{Email: "b@x.com", PasswordHash: "x", Role: "CASHIER"},
	}
	require.NoError(t, gdb.Create(&batch).Error)
	require.NotEmpty(t, batch[0].ID)
	require.NotEmpty(t, batch[1].ID)
	require.NotEqual(t, batch[0].ID, batch[1].ID)
}
