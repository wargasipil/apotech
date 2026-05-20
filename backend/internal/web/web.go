// Package web serves the React SPA embedded into the binary at build time.
// The build pipeline (`make build`, the Dockerfile, the Windows build script)
// copies frontend/dist into ./dist before `go build`; this makes the single
// self-contained binary serve both the UI and the API on one origin.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// all: is required so the committed stub (.gitkeep, a dotfile) is embedded and
// the directive compiles on a fresh checkout where no real build has run yet.
//
//go:embed all:dist
var assets embed.FS

// notBuilt is shown when the binary was compiled without a real frontend build
// (only the committed .gitkeep is present). Shipped artifacts always embed the
// real SPA, so this is dev-only; use the Vite dev server (`make web`) instead.
const notBuilt = `<!doctype html><html><body style="font-family:sans-serif;padding:2rem">
<h1>Apotech</h1><p>Frontend not built into this binary.</p>
<p>Run <code>make build</code> to embed the SPA, or use the Vite dev server (<code>make web</code>).</p>
</body></html>`

// Handler serves the embedded SPA. Existing asset paths are served directly
// (FileServer sets content-type + caching); unknown non-asset paths fall back
// to index.html so client-side routes (createBrowserRouter) resolve on a hard
// refresh. Missing /assets/* return a real 404 rather than HTML.
func Handler() http.Handler {
	dist, err := fs.Sub(assets, "dist")
	if err != nil {
		panic(err) // embed guarantees the dist dir exists at compile time
	}
	fileServer := http.FileServer(http.FS(dist))
	index, indexErr := fs.ReadFile(dist, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if upath == "" {
			upath = "index.html"
		}
		// Serve a real embedded file when it exists.
		if f, ferr := dist.Open(upath); ferr == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// Missing hashed asset → real 404 (don't hand back HTML for a .js/.css).
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			http.NotFound(w, r)
			return
		}
		// SPA fallback.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if indexErr != nil {
			_, _ = w.Write([]byte(notBuilt))
			return
		}
		_, _ = w.Write(index)
	})
}
