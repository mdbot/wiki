package main

import (
	"io/fs"
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


type ErrorPageArgs struct {
	CommonPageArgs
	ShowLoginForm bool
	OldPageTitle  string
}

func PageErrorHandler(templateFs fs.FS) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fakeWriter := &notFoundInterceptWriter{realWriter: w}

			next.ServeHTTP(fakeWriter, r)

			if fakeWriter.status == http.StatusNotFound {
				isWiki := false
				if strings.HasPrefix(r.RequestURI, "/view/") || strings.HasPrefix(r.RequestURI, "/history") {
					isWiki = true
				}
				oldPageTitle := ""
				if strings.HasPrefix(r.RequestURI, "/view/") {
					oldPageTitle = strings.TrimPrefix(r.RequestURI, "/view/")
				}
				renderTemplate(templateFs, NotFound, http.StatusNotFound, w, &ErrorPageArgs{
					CommonPageArgs{
						Session:      getSessionArgs(w, r),
						PageTitle:    "Page not found",
						IsWikiPage:   isWiki,
						IsError: true,
					},
					false,
					oldPageTitle,
				})
			}
			if fakeWriter.status == http.StatusUnauthorized {
				renderTemplate(templateFs, Unauthorized, http.StatusUnauthorized, w, &ErrorPageArgs{
					CommonPageArgs{
						Session:   getSessionArgs(w, r),
						PageTitle: "Unauthorized",
						IsError: true,
					},
					true,
					"",
				})
			}
			if fakeWriter.status == http.StatusForbidden {
				renderTemplate(templateFs, Forbidden, http.StatusForbidden, w, &ErrorPageArgs{
					CommonPageArgs{
						Session:   getSessionArgs(w, r),
						PageTitle: "Forbidden",
						IsError: true,
					},
					false,
					"",
				})
			}
			if fakeWriter.status == http.StatusInternalServerError {
				renderTemplate(templateFs, ServerError, http.StatusInternalServerError, w, &ErrorPageArgs{
					CommonPageArgs{
						Session:   getSessionArgs(w, r),
						PageTitle: "Server Error",
						IsError: true,
					},
					false,
					"",
				})
			}
		})
	}
}
