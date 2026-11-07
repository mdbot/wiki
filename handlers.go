package main

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/gorilla/handlers"
)

func RedirectMainPageHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Add("Location", fmt.Sprintf("/view/%s", *mainPage))
		writer.WriteHeader(http.StatusSeeOther)
	}
}

func LoggingHandler(dst io.Writer) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return handlers.LoggingHandler(dst, h)
	}
}

func LowerCaseCanonical(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		url := request.URL
		url.Path = strings.ToLower(url.Path)
		if url.RequestURI() != request.RequestURI {
			http.Redirect(writer, request, url.RequestURI(), http.StatusPermanentRedirect)
		} else {
			next.ServeHTTP(writer, request)
		}
	})
}

func StripSlashes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/" && strings.HasSuffix(request.URL.Path, "/") {
			http.Redirect(writer, request, strings.TrimSuffix(request.URL.Path, "/"), http.StatusPermanentRedirect)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

// CleanPath redirects requests whose decoded path contains "." or ".." segments or repeated slashes to the
// canonical path, so later handlers (in particular StripSlashes) never see them.
func CleanPath(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			p := request.URL.Path
			if p == "" || p[0] != '/' {
				p = "/" + p
			}
			clean := path.Clean(p)
			if strings.HasSuffix(p, "/") && clean != "/" {
				clean += "/"
			}

			if clean != request.URL.Path {
				u := *request.URL
				u.Path = clean
				writer.Header().Set("Location", u.String())
				writer.WriteHeader(http.StatusMovedPermanently)
				return
			}
			next.ServeHTTP(writer, request)
		},
	)
}
