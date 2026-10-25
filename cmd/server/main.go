// Command server runs the fh-items web server: it loads the embedded
// catalog, builds a fresh in-memory sample runtime state, and serves the
// static frontend assets. The API and persistent save/load are later
// milestones; this is scaffolding to prove the pipeline end-to-end.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"

	"fh-items/internal/catalog"
	"fh-items/internal/state"
	"fh-items/web"
)

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	flag.Parse()

	cat, err := catalog.Load()
	if err != nil {
		log.Fatalf("loading catalog: %v", err)
	}

	s := state.New(cat)
	if err := s.Validate(); err != nil {
		log.Fatalf("sample state failed validation: %v", err)
	}

	log.Printf("loaded catalog: %d items (schema v%d, source checksum %s)",
		len(cat.Items), cat.SchemaVersion, cat.SourceChecksum)
	log.Printf("built sample state: schema v%d, %d items tracked", s.SchemaVersion, len(s.Items))

	staticFS, err := fs.Sub(web.Static, "static")
	if err != nil {
		log.Fatalf("preparing static assets: %v", err)
	}
	http.Handle("/", http.FileServer(http.FS(staticFS)))

	fmt.Printf("listening on %s\n", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal(err)
	}
}
