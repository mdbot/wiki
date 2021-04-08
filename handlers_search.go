package main

import (
	"net/http"
)

type SearchRequest interface {
	SearchWiki(pattern string) []SearchResult
}

func SearchHandler(templates *Templates, backend SearchRequest) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pattern := r.FormValue("pattern")
		var results []SearchResult
		if pattern != "" {
			results = backend.SearchWiki(pattern)
		}
		templates.RenderSearch(w, r, pattern, results)
	}
}
