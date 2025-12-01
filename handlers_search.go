package main

import (
	"log"
	"net/http"
)

type SearchRequest interface {
	SearchWiki(pattern string) []SearchResult
}

func SearchHandler(templates *Templates, backend SearchRequest) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			log.Printf("Error parsing form: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		pattern := r.FormValue("pattern")
		var results []SearchResult
		if pattern != "" {
			results = backend.SearchWiki(pattern)
		}
		templates.RenderSearch(w, r, pattern, results)
	}
}
