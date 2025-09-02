package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/stonebraker/lap/apps/server/internal/httpx"

	"github.com/go-chi/chi/v5"
)

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	dir := flag.String("dir", "apps/server/static/publisherapi", "directory to serve")
	flag.Parse()

	// Serve .htmx files as HTML
	_ = mime.AddExtensionType(".htmx", "text/html; charset=utf-8")

	mux := chi.NewRouter()
	
	// Add PUT handler for posts before mounting static router
	mux.Put("/people/alice/posts/{postID}/", func(w http.ResponseWriter, r *http.Request) {
		handlePostUpdate(w, r, *dir)
	})
	
	// Add PUT handler for resource attestations
	mux.Put("/people/alice/posts/{postID}/_la_resource.json", func(w http.ResponseWriter, r *http.Request) {
		handleResourceAttestationUpdate(w, r, *dir)
	})
	
	mux.Mount("/", httpx.NewStaticRouter(*dir))

	log.Printf("publisherapi serving %s on %s", *dir, *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(fmt.Errorf("server error: %w", err))
	}
}

// handlePostUpdate handles PUT requests to update post fragments
func handlePostUpdate(w http.ResponseWriter, r *http.Request, baseDir string) {
	postID := chi.URLParam(r, "postID")
	if postID == "" {
		http.Error(w, "Post ID is required", http.StatusBadRequest)
		return
	}
	
	// Read the fragment content from request body
	fragmentContent, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	
	// Construct the file path
	fragmentPath := filepath.Join(baseDir, "people", "alice", "posts", postID, "index.htmx")
	
	// Ensure the directory exists
	dir := filepath.Dir(fragmentPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		http.Error(w, "Failed to create directory", http.StatusInternalServerError)
		return
	}
	
	// Write the fragment to the file
	if err := os.WriteFile(fragmentPath, fragmentContent, 0644); err != nil {
		http.Error(w, "Failed to write fragment", http.StatusInternalServerError)
		return
	}
	
	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"success": true, "message": "Fragment updated successfully", "path": "/people/alice/posts/%s/"}`, postID)
}

// handleResourceAttestationUpdate handles PUT requests to update resource attestation files
func handleResourceAttestationUpdate(w http.ResponseWriter, r *http.Request, baseDir string) {
	postID := chi.URLParam(r, "postID")
	if postID == "" {
		http.Error(w, "Post ID is required", http.StatusBadRequest)
		return
	}
	
	// Read the attestation content from request body
	attestationContent, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	
	// Construct the file path
	attestationPath := filepath.Join(baseDir, "people", "alice", "posts", postID, "_la_resource.json")
	
	// Ensure the directory exists
	dir := filepath.Dir(attestationPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		http.Error(w, "Failed to create directory", http.StatusInternalServerError)
		return
	}
	
	// Write the attestation to the file
	if err := os.WriteFile(attestationPath, attestationContent, 0644); err != nil {
		http.Error(w, "Failed to write resource attestation", http.StatusInternalServerError)
		return
	}
	
	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"success": true, "message": "Resource attestation updated successfully", "path": "/people/alice/posts/%s/_la_resource.json"}`, postID)
}
