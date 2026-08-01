package promptsheon

import (
	"io/fs"
	"net/http"
	"strings"
)

// HandleFrontend registers a catch-all route that serves the
// provided frontend filesystem as an SPA. Call AFTER all API
// routes so /api/* takes priority. Serves static files and
// falls back to index.html for unmatched paths.
func (s *Server) HandleFrontend(frontend fs.FS) {
	fileServer := http.FileServer(http.FS(frontend))
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if f, err := fs.Stat(frontend, path); err == nil && !f.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
