package main

import (
	"embed"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/gorilla/handlers"
	"github.com/microcosm-cc/bluemonday"
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

func NewLoggingHandler(dst io.Writer) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return handlers.LoggingHandler(dst, h)
	}
}

func NotFoundHandler(h http.Handler, files fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fakeWriter := &notFoundInterceptWriter{realWriter: w}
		h.ServeHTTP(fakeWriter, r)
		if fakeWriter.status == http.StatusNotFound {
			errorFile, err := files.Open("404.html")
			if err != nil {
				log.Printf("Unable to output 404: %s", err.Error())
				http.NotFound(w, r)
				return
			}
			errorbytes, err := io.ReadAll(errorFile)
			if err != nil {
				log.Printf("Unable to output 404: %s", err.Error())
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			_, err = w.Write(errorbytes)
			if err != nil {
				log.Printf("Unable to output 404: %s", err.Error())
			}
		}
	}
}

func GetEmbedOrOSFS(path string, embedFs embed.FS) (fs.FS, error) {
	_, err := os.Stat(path)
	if err == nil {
		return os.DirFS(path), nil
	}
	_, err = embedFs.Open(path)
	if err != nil {
		return nil, err
	}
	staticFiles, err := fs.Sub(embedFs, path)
	if err != nil {
		return nil, err
	}
	return staticFiles, nil
}

type PageProvider interface {
	GetPage(title string) (*Page, error)
}

func RenderPageHandler(templateFs fs.FS, pp PageProvider) http.HandlerFunc {
	type LastModifiedDetails struct {
		User string
		Time time.Time
	}

	type RenderPageArgs struct {
		PageTitle    string
		PageContent  template.HTML
		CanEdit      bool
		LastModified LastModifiedDetails
	}

	renderTpl := template.Must(template.ParseFS(templateFs, "index.html"))

	return func(writer http.ResponseWriter, request *http.Request) {
		pageTitle := strings.TrimPrefix(request.URL.Path, "/view/")

		page, err := pp.GetPage(pageTitle)
		if err != nil {
			writer.WriteHeader(http.StatusNotFound)
			return
		}

		unsafe := markdown.ToHTML([]byte(page.Content), nil, nil)
		html := bluemonday.UGCPolicy().SanitizeBytes(unsafe)

		if err := renderTpl.Execute(writer, RenderPageArgs{
			PageTitle:   pageTitle,
			CanEdit:     false,
			PageContent: template.HTML(html),
			LastModified: LastModifiedDetails{
				User: page.LastModified.User,
				Time: page.LastModified.Time,
			},
		}); err != nil {
			// TODO: We should probably send an error to the client
			log.Printf("Error rendering template: %v\n", err)
		}
	}
}
