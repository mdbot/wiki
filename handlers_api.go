package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type Lister interface {
	ListPages() ([]string, error)
	ListFiles() ([]File, error)
}

func ApiListHandler(l Lister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var res []string

		if err := r.ParseForm(); err != nil {
			log.Printf("Error parsing form: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if r.FormValue("type") == "file" {
			files, err := l.ListFiles()
			if err != nil {
				log.Printf("Failed to list files: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			for i := range files {
				res = append(res, files[i].Name)
			}
		} else {
			pages, err := l.ListPages()
			if err != nil {
				log.Printf("Failed to list pages: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			res = pages
		}

		b, err := json.Marshal(res)
		if err != nil {
			log.Printf("Failed to marshal list contents: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, _ = w.Write(b)
	}
}
