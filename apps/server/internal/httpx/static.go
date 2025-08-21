package httpx

import (
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

// NewStaticRouter returns an http.Handler that serves files from baseDir.
// If the request path resolves to a directory (with or without trailing slash),
// it will serve index.html, then index.json if present, in that precedence order.
func NewStaticRouter(baseDir string) http.Handler {
	r := chi.NewRouter()

	r.HandleFunc("/*", func(w http.ResponseWriter, req *http.Request) {
		// Clean and ensure path is rooted
		cleanPath := path.Clean("/" + req.URL.Path)

		absBase, err := filepath.Abs(baseDir)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		absPath, err := filepath.Abs(filepath.Join(absBase, cleanPath))
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if !strings.HasPrefix(absPath, absBase) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		info, statErr := os.Stat(absPath)
		if statErr == nil {
			if info.IsDir() {
				serveIndex(w, req, absPath)
				return
			}
			// If serving an .htmx file, set Content-Type explicitly
			if strings.HasSuffix(strings.ToLower(absPath), ".htmx") {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
			}
			w.Header().Set("Cache-Control", "no-store")
			http.ServeFile(w, req, absPath)
			return
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		// If the exact path does not exist, but the cleaned path could be a directory
		// without trailing slash, still attempt to serve index files.
		if tryDir := absPath; true {
			if dInfo, dErr := os.Stat(tryDir); dErr == nil && dInfo.IsDir() {
				serveIndex(w, req, tryDir)
				return
			}
		}

		http.NotFound(w, req)
	})

	return r
}

func serveIndex(w http.ResponseWriter, req *http.Request, dir string) {
	indexHTML := filepath.Join(dir, "index.html")
	if _, err := os.Stat(indexHTML); err == nil {
		http.ServeFile(w, req, indexHTML)
		return
	}
	// Support index.htmx as a directory index
	indexHTMX := filepath.Join(dir, "index.htmx")
	if _, err := os.Stat(indexHTMX); err == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeFile(w, req, indexHTMX)
		return
	}
	indexJSON := filepath.Join(dir, "index.json")
	if _, err := os.Stat(indexJSON); err == nil {
		http.ServeFile(w, req, indexJSON)
		return
	}
	http.NotFound(w, req)
}
