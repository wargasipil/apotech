package service

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// PgToolsVersion pins the EDB Windows binary zip that resolvePgDump auto-fetches.
// **Keep in sync with packaging/windows/build-windows.ps1's $PgVersion constant** —
// the Windows installer ships the same zip at build time so the in-app feature
// and the installer agree on a version.
const PgToolsVersion = "16.4-1"

// pgDumpDownloadURL is the EDB official binaries zip for Windows x64. Matches
// the URL pattern packaging/windows/build-windows.ps1 uses to build the
// installer. HTTPS, no checksum (same supply-chain posture as the installer).
func pgDumpDownloadURL() string {
	return "https://get.enterprisedb.com/postgresql/postgresql-" +
		PgToolsVersion + "-windows-x64-binaries.zip"
}

// pgDumpBinName returns "pg_dump.exe" on Windows, "pg_dump" elsewhere.
func pgDumpBinName() string {
	if runtime.GOOS == "windows" {
		return "pg_dump.exe"
	}
	return "pg_dump"
}

// resolvePgDump returns an absolute path to a usable pg_dump binary, searching
// in this priority order:
//
//  1. PATH (`exec.LookPath`).
//  2. Bundled next to the apotech executable (the Windows installer ships
//     pg_dump.exe at <appdir>/pgsql/bin/ but does NOT put it on PATH — this
//     branch catches it).
//  3. A previously-cached download under <pgToolsDir>/pgsql/bin/.
//  4. Windows + autoFetch only: download EDB's official binaries zip, extract
//     pgsql/ into <pgToolsDir>/, return the path.
//  5. Otherwise: a friendly error with install instructions for the current OS.
//
// The resolver is idempotent — a cached binary is found at step 3 on every
// subsequent call.
func resolvePgDump(ctx context.Context, pgToolsDir string, autoFetch bool) (string, error) {
	binName := pgDumpBinName()

	// 1. PATH.
	if p, err := exec.LookPath("pg_dump"); err == nil {
		return p, nil
	}

	// 2. Bundled next to the running apotech binary (Windows installer layout).
	if exePath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exePath), "pgsql", "bin", binName)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate, nil
		}
	}

	// 3. Previously-cached auto-download.
	if pgToolsDir != "" {
		candidate := filepath.Join(pgToolsDir, "pgsql", "bin", binName)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate, nil
		}
	}

	// 4. Auto-fetch (Windows only, opt-in).
	if autoFetch && runtime.GOOS == "windows" && pgToolsDir != "" {
		slog.Info("pg_dump not found; auto-downloading EDB Windows binaries",
			"version", PgToolsVersion, "dest", pgToolsDir)
		if err := downloadAndExtractPgTools(ctx, pgToolsDir); err != nil {
			return "", fmt.Errorf("auto-download pg_dump: %w", err)
		}
		candidate := filepath.Join(pgToolsDir, "pgsql", "bin", binName)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			slog.Info("pg_dump installed", "path", candidate)
			return candidate, nil
		}
		return "", errors.New("auto-download completed but pg_dump still not found in the extracted zip")
	}

	return "", errors.New("pg_dump not found. " + installHintForOS())
}

// installHintForOS returns a friendly platform-specific install instruction.
func installHintForOS() string {
	switch runtime.GOOS {
	case "linux":
		return "Install postgresql-client (e.g. `apt install postgresql-client`)."
	case "darwin":
		return "Install postgresql-client (e.g. `brew install libpq` then add libpq's bin to PATH)."
	case "windows":
		return "Install PostgreSQL or use the Apotech Windows installer (which bundles it)."
	default:
		return "Install postgresql-client for your platform."
	}
}

// downloadAndExtractPgTools fetches the EDB Windows binaries zip into a cache
// file under pgToolsDir, then extracts just the `pgsql/` subtree (binaries +
// dependent DLLs) into pgToolsDir. The cached zip stays on disk so a subsequent
// fresh extract (e.g. corrupted cache) doesn't re-download.
//
// Retries downloadToFile up to 3 times — EDB's server has been observed to
// drop long-lived downloads, but downloadToFile preserves .tmp on failure and
// uses Range on the next attempt to resume from the saved offset. Each retry
// makes net progress (or finishes the file).
func downloadAndExtractPgTools(ctx context.Context, pgToolsDir string) error {
	if err := os.MkdirAll(pgToolsDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", pgToolsDir, err)
	}
	url := pgDumpDownloadURL()
	zipPath := filepath.Join(pgToolsDir, "postgresql-"+PgToolsVersion+"-windows-x64-binaries.zip")

	if _, err := os.Stat(zipPath); errors.Is(err, os.ErrNotExist) {
		const maxAttempts = 3
		var lastErr error
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			slog.Info("downloading pg tools",
				"attempt", attempt, "max", maxAttempts, "url", url)
			if err := downloadToFile(ctx, url, zipPath); err == nil {
				lastErr = nil
				break
			} else {
				slog.Warn("pg tools download attempt failed (will resume on retry)",
					"attempt", attempt, "error", err.Error())
				lastErr = err
				// Short backoff before next try; honor caller cancellation.
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(2 * time.Second):
				}
			}
		}
		if lastErr != nil {
			return lastErr
		}
	} else if err != nil {
		return err
	}
	return extractZipSubtree(zipPath, pgToolsDir, "pgsql/")
}

// downloadToFile streams a URL to disk, resuming from the existing .tmp size
// if one is present. Writes to <path>.tmp first and renames on success — and
// **preserves .tmp on mid-stream failure** so a subsequent call resumes via
// HTTP Range from where we left off. EDB's CDN serves Range requests, but the
// connection can be reset by the remote on long transfers (~40 min on a slow
// foreign link) — resume + retry is what makes the auto-download usable.
//
// Returns nil only after a complete download has been successfully renamed.
func downloadToFile(ctx context.Context, url, path string) error {
	tmp := path + ".tmp"
	var offset int64
	if st, err := os.Stat(tmp); err == nil {
		offset = st.Size()
	}

	// Generous per-attempt timeout — slow networks have been observed taking
	// ~40 min for the 75 MB zip before EDB resets the connection. The retry
	// loop in downloadAndExtractPgTools wraps this so a drop just resumes.
	cli := &http.Client{Timeout: 60 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := cli.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	var f *os.File
	switch resp.StatusCode {
	case http.StatusOK:
		// Server ignored Range (or we asked from 0) → start fresh.
		f, err = os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		offset = 0
	case http.StatusPartialContent:
		// Continue from the byte offset.
		f, err = os.OpenFile(tmp, os.O_WRONLY|os.O_APPEND, 0o644)
	default:
		return fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	if err != nil {
		return err
	}

	written, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		// Preserve .tmp so the next call resumes (the bytes that did land
		// are still on disk; the next Range request continues from there).
		return fmt.Errorf("write %s (resumed from %d, wrote %d more): %w",
			tmp, offset, written, copyErr)
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp, path)
}

// extractZipSubtree extracts entries under prefix (e.g. "pgsql/") into destDir.
// Other entries (docs/, doc/, symbols/) are skipped to keep the cache small.
// Refuses entries whose normalized path escapes destDir (Zip-Slip).
func extractZipSubtree(zipPath, destDir, prefix string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		name := f.Name
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		// Zip-Slip guard: clean and ensure the result is within destDir.
		target := filepath.Join(destDir, filepath.FromSlash(name))
		clean := filepath.Clean(target)
		if !strings.HasPrefix(clean, filepath.Clean(destDir)+string(os.PathSeparator)) &&
			clean != filepath.Clean(destDir) {
			return fmt.Errorf("zip entry escapes dest: %q", name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(clean, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(clean), 0o755); err != nil {
			return err
		}
		if err := extractZipFile(f, clean); err != nil {
			return fmt.Errorf("extract %s: %w", name, err)
		}
	}
	return nil
}

func extractZipFile(zf *zip.File, dst string) error {
	src, err := zf.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, zf.Mode())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
