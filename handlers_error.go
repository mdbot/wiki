package main

import (
	"net/http"
	"strings"
)

type errorInterceptingWriter struct {
	realWriter http.ResponseWriter
	status     int
}

func (w *errorInterceptingWriter) Header() http.Header {
	return w.realWriter.Header()
}

func (w *errorInterceptingWriter) WriteHeader(status int) {
	w.status = status
	if w.shouldProxy() {
		w.realWriter.WriteHeader(status)
	}
}

func (w *errorInterceptingWriter) Write(p []byte) (int, error) {
	if w.shouldProxy() {
		return w.realWriter.Write(p)
	}
	return len(p), nil
}

func (w *errorInterceptingWriter) shouldProxy() bool {
	return w.status != http.StatusNotFound &&
		w.status != http.StatusUnauthorized &&
		w.status != http.StatusForbidden &&
		w.status != http.StatusInternalServerError &&
		w.status != http.StatusBadRequest
}

func PageErrorHandler(t *Templates) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fakeWriter := &errorInterceptingWriter{realWriter: w}

			next.ServeHTTP(fakeWriter, r)

			switch fakeWriter.status {
			case http.StatusNotFound:
				isWiki := strings.HasPrefix(r.RequestURI, "/view/") || strings.HasPrefix(r.RequestURI, "/history")
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
			case http.StatusBadRequest:
				t.RenderBadRequest(w, r)
			}
		})
	}
}
