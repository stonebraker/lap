package main

import (
	"flag"
	"fmt"
	"log"
	"mime"
	"net/http"

	"github.com/stonebraker/lap/apps/server/internal/httpx"

	"github.com/go-chi/chi/v5"
)

func main() {
	addr := flag.String("addr", ":8081", "address to listen on")
	dir := flag.String("dir", "apps/server/static/publisherapi", "directory to serve")
	flag.Parse()

	// Serve .htmx files as HTML
	_ = mime.AddExtensionType(".htmx", "text/html; charset=utf-8")

	mux := chi.NewRouter()
	mux.Mount("/", httpx.NewStaticRouter(*dir))

	log.Printf("publisherapi serving %s on %s", *dir, *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(fmt.Errorf("server error: %w", err))
	}
}
