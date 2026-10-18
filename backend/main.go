package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	line := NewLineClient(cfg.LineChannelID, cfg.LineChannelToken)
	images := NewImageStore(2 * time.Hour)
	srv := NewServer(cfg, line, images)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", srv.handleHealth)
	mux.HandleFunc("POST /api/complaints", srv.handleCreateComplaint)
	mux.HandleFunc("GET /images/{id}", srv.handleGetImage)
	registerStatic(mux, cfg.StaticDir)

	addr := ":" + cfg.Port
	log.Printf("listening on %s (static dir: %s, public base url: %s)", addr, cfg.StaticDir, cfg.PublicBaseURL)
	if err := http.ListenAndServe(addr, logRequests(mux)); err != nil {
		log.Fatal(err)
	}
}

// registerStatic serves the built React/LIFF frontend. The frontend is a
// single-page app, so any path that isn't a real static file falls back
// to index.html and lets client-side routing (if any) take over.
func registerStatic(mux *http.ServeMux, dir string) {
	fileServer := http.FileServer(http.Dir(dir))
	indexPath := filepath.Join(dir, "index.html")

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		cleanPath := filepath.Clean(r.URL.Path)
		fullPath := filepath.Join(dir, cleanPath)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, indexPath)
	})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
