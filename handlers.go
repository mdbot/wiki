package main

import (
	"fmt"
	"io"
	"net/http"
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
		if strings.ToLower(request.RequestURI) != request.RequestURI {
			http.Redirect(writer, request, strings.ToLower(request.RequestURI), http.StatusPermanentRedirect)
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
