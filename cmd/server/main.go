// Command server serves the fh-items crafting planner. Nothing is persisted
// server-side: building levels and discovered items come from a cookie on
// each request.
package main

import (
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"

	"fh-items/internal/catalog"
	"fh-items/web"
)

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	flag.Parse()

	cat, err := catalog.Load()
	if err != nil {
		log.Fatalf("loading catalog: %v", err)
	}
	log.Printf("loaded catalog: %d items (schema v%d, source checksum %s)",
		len(cat.Items), cat.SchemaVersion, cat.SourceChecksum)

	ghItems, err := catalog.LoadGloomhaven()
	if err != nil {
		log.Fatalf("loading gloomhaven items: %v", err)
	}

	indexTmpl, err := template.ParseFS(web.Templates, "templates/index.html.tmpl")
	if err != nil {
		log.Fatalf("parsing index template: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if storeShareInCookie(w, r) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		data := buildPageData(cat, ghItems, unlocksFromRequest(r))
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
