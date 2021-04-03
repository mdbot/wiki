package main

import (
	"net/http"
	"strings"
)

type notFoundInterceptWriter struct {
	realWriter http.ResponseWriter
	status     int
}

func (w *notFoundInterceptWriter) Header() http.Header {
	return w.realWriter.Header()
}

func (w *notFoundInterceptWriter) WriteHeader(status int) {
	w.status = status
	if status != http.StatusNotFound {
		w.realWriter.WriteHeader(status)
	}
}

func (w *notFoundInterceptWriter) Write(p []byte) (int, error) {
	if w.status != http.StatusNotFound {
		return w.realWriter.Write(p)
	}
	return len(p), nil
}

func PageErrorHandler(t *Templates) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fakeWriter := &notFoundInterceptWriter{realWriter: w}

			next.ServeHTTP(fakeWriter, r)

			switch fakeWriter.status {
			case http.StatusNotFound:
				isWiki := false
				if strings.HasPrefix(r.RequestURI, "/view/") || strings.HasPrefix(r.RequestURI, "/history") {
					isWiki = true
				}
				oldPageTitle := ""
				if strings.HasPrefix(r.RequestURI, "/view/") {
					oldPageTitle = strings.TrimPrefix(r.RequestURI, "/view/")
				}
				t.RenderNotFound(w, r, isWiki, oldPageTitle)
			case http.StatusUnauthorized:
				t.RenderUnauthorised(w, r)
			case http.StatusForbidden:
				t.RenderForbidden(w, r)
			case http.StatusInternalServerError:
				t.RenderInternalError(w, r)
			}
		})
	}
}
