package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// distFS holds the compiled, minified frontend bundle. The dist directory is
// committed to the repository so that pure-Go builds (including `go install`)
// work without a Node toolchain. Run `make build-web` to regenerate it.
//
//go:embed all:dist
var distFS embed.FS

// staticHandler serves the embedded SPA. Unknown non-asset paths fall back to
// index.html so client-side routing works.
func staticHandler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if reqPath == "" {
			reqPath = "index.html"
		}

		if _, err := fs.Stat(sub, reqPath); err != nil {
			// Not a real asset: serve the SPA entry point.
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}
