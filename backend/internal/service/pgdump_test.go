package service

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestResolvePgDump_PrefersCache pins the resolver's cache priority: with
// PATH cleared, no bundled-next-to-exe binary, and autoFetch disabled, a
// pg_dump (or pg_dump.exe) staged at <pgToolsDir>/pgsql/bin/ is returned.
//
// The actual auto-download branch isn't covered here — it'd require network
// access (a 75 MB download from EDB), defeating test isolation. Manual smoke
// in DEPLOYMENT.md / CLAUDE.md is the verification for that path.
func TestResolvePgDump_PrefersCache(t *testing.T) {
	pgToolsDir := t.TempDir()
	binDir := filepath.Join(pgToolsDir, "pgsql", "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))

	binName := "pg_dump"
	if runtime.GOOS == "windows" {
		binName = "pg_dump.exe"
	}
	staged := filepath.Join(binDir, binName)
	require.NoError(t, os.WriteFile(staged, []byte{}, 0o755))

	// Force PATH to a directory that contains no pg_dump so step 1 always
	// fails. Use the test's temp dir as the empty PATH entry (guaranteed to
	// have no pg_dump in it). t.Setenv restores PATH at test end.
	t.Setenv("PATH", t.TempDir())

	got, err := resolvePgDump(context.Background(), pgToolsDir, false)
	require.NoError(t, err)
	require.Equal(t, staged, got, "resolver must return the cached path")

	// Idempotent: a second call returns the same path without flakes.
	got2, err := resolvePgDump(context.Background(), pgToolsDir, false)
	require.NoError(t, err)
	require.Equal(t, staged, got2)
}

// TestResolvePgDump_MissingErrorIsActionable proves the "nothing found and
// autoFetch off" branch returns an OS-specific install hint, not a stack trace.
func TestResolvePgDump_MissingErrorIsActionable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	pgToolsDir := t.TempDir() // empty, no cache

	_, err := resolvePgDump(context.Background(), pgToolsDir, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "pg_dump not found")
	// Hint must mention SOMETHING the user can act on for the current OS.
	switch runtime.GOOS {
	case "linux":
		require.Contains(t, err.Error(), "postgresql-client")
	case "darwin":
		require.Contains(t, err.Error(), "libpq")
	case "windows":
		require.Contains(t, err.Error(), "PostgreSQL")
	}
}
