package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
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

func unauthorized(w http.ResponseWriter, realm string) {
	w.Header().Add("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, realm))
	w.WriteHeader(http.StatusUnauthorized)
}

func BasicAuthHandler(realm string, credentials map[string]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()
			if !ok {
				unauthorized(w, realm)
				return
			}
			_, ok = credentials[username]
			if ok && credentials[username] == password {
				next.ServeHTTP(w, r)
				return
			}
			unauthorized(w, realm)
		})
	}
}

func BasicAuthFromEnv() func(http.Handler) http.Handler {
	if *realm == "" {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r)
			})
		}
	}
	return BasicAuthHandler(*realm, map[string]string{*username: *password})
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

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)

	return func(writer http.ResponseWriter, request *http.Request) {
		pageTitle := strings.TrimPrefix(request.URL.Path, "/view/")

		page, err := pp.GetPage(pageTitle)
		if err != nil {
			notFoundTpl := template.Must(template.ParseFS(templateFs, "notfound.html"))
			writer.WriteHeader(http.StatusNotFound)
			if err := notFoundTpl.Execute(writer, &RenderPageArgs{
				PageTitle: pageTitle,
				CanEdit:   true,
			}); err != nil {
				log.Printf("Error rendering template: %v\n", err)
			}
			return
		}

		b := &bytes.Buffer{}
		if err := md.Convert([]byte(page.Content), b); err != nil {
			log.Printf("Failed to render markdown: %v\n", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}

		renderTpl := template.Must(template.ParseFS(templateFs, "index.html"))
		if err := renderTpl.Execute(writer, RenderPageArgs{
			PageTitle:   pageTitle,
			CanEdit:     true,
			PageContent: template.HTML(b.String()),
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

func EditPageHandler(templateFs fs.FS, pp PageProvider) http.HandlerFunc {
	type EditPageArgs struct {
		PageTitle   string
		PageContent string
		CanEdit     bool
	}

	return func(writer http.ResponseWriter, request *http.Request) {
		pageTitle := strings.TrimPrefix(request.URL.Path, "/edit/")

		var content string
		if page, err := pp.GetPage(pageTitle); err == nil {
			content = page.Content
		}

		editTpl := template.Must(template.ParseFS(templateFs, "edit.html"))
		if err := editTpl.Execute(writer, EditPageArgs{
			PageTitle:   pageTitle,
			CanEdit:     true,
			PageContent: content,
		}); err != nil {
			// TODO: We should probably send an error to the client
			log.Printf("Error rendering template: %v\n", err)
		}
	}
}

type PageEditor interface {
	PutPage(title string, content []byte, user string, message string) error
}

func SubmitPageHandler(pe PageEditor) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		pageTitle := strings.TrimPrefix(request.URL.Path, "/edit/")

		content := request.FormValue("content")
		message := request.FormValue("message")
		user := "Anonymoose"
		if len(*realm) > 0 {
			user, _, _ = request.BasicAuth()
		}

		if err := pe.PutPage(pageTitle, []byte(content), user, message); err != nil {
			// TODO: We should probably send an error to the client
			log.Printf("Error saving page: %v\n", err)
		} else {
			writer.Header().Add("Location", fmt.Sprintf("/view/%s", pageTitle))
			writer.WriteHeader(http.StatusSeeOther)
		}
	}
}

func RedirectMainPageHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Add("Location", fmt.Sprintf("/view/%s", *mainPage))
		writer.WriteHeader(http.StatusSeeOther)
	}
}
