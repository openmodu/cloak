package http

import (
	"embed"
	"net/http"
)

//go:embed web/index.html web/app.css web/app.js
var webFiles embed.FS

// Only the static shell is public. API calls retain their existing authorization.
func registerWeb(mux *http.ServeMux) {
	for route, asset := range map[string]struct{ path, contentType string }{
		"GET /{$}":            {"web/index.html", "text/html; charset=utf-8"},
		"GET /assets/app.css": {"web/app.css", "text/css; charset=utf-8"},
		"GET /assets/app.js":  {"web/app.js", "text/javascript; charset=utf-8"},
	} {
		data, err := webFiles.ReadFile(asset.path)
		if err != nil {
			panic(err)
		}
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", asset.contentType)
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
			_, _ = w.Write(data)
		})
	}
}
