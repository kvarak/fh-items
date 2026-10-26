// Command server runs the fh-items web server: it loads the embedded
// catalog, builds a fresh default runtime state, and serves the static
// frontend assets plus the JSON API (GET /api/state, POST /api/check).
// Nothing is persisted server-side - building levels, discovered items,
// and resources all live client-side (cookies), read per-request.
package main

import (
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"

	"fh-items/internal/api"
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
		log.Fatalf("default state failed validation: %v", err)
	}

	log.Printf("loaded catalog: %d items (schema v%d, source checksum %s)",
		len(cat.Items), cat.SchemaVersion, cat.SourceChecksum)

	indexTmpl, err := template.ParseFS(web.Templates, "templates/index.html.tmpl")
	if err != nil {
		log.Fatalf("parsing index template: %v", err)
	}

	apiServer := api.NewServer(cat, s)

	mux := http.NewServeMux()
	apiServer.RegisterRoutes(mux)

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		st := stateFromRequest(r, apiServer.Catalog, apiServer.State())
		data := buildPageData(apiServer.Catalog, st)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := indexTmpl.Execute(w, data); err != nil {
			log.Printf("rendering index page: %v", err)
		}
	})

	staticFS, err := fs.Sub(web.Static, "static")
	if err != nil {
		log.Fatalf("preparing static assets: %v", err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	fmt.Printf("listening on %s\n", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
