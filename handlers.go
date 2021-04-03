package main

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/mdbot/wiki/config"
)

type TemplateName string

const (
	NotFound        TemplateName = "404"
	Unauthorized    TemplateName = "error"
	Forbidden       TemplateName = "error"
	ServerError     TemplateName = "error"
	EditPage        TemplateName = "edit"
	HistoryPage     TemplateName = "history"
	ViewPage        TemplateName = "index"
	ListPage        TemplateName = "list"
	ListFilesPage   TemplateName = "listfiles"
	UploadPage      TemplateName = "upload"
	ManageUsersPage TemplateName = "users"
	RenamePage      TemplateName = "rename"
	DeletePage      TemplateName = "delete"
)

type CommonPageArgs struct {
	Session      SessionArgs
	PageTitle    string
	IsWikiPage   bool
	LastModified *LastModifiedDetails
	IsError      bool
}

type SessionArgs struct {
	CanEdit      bool
	Error        string
	Notice       string
	User         *config.User
	CsrfField    template.HTML
	RequestedUrl string
}

type LastModifiedDetails struct {
	User string
	Time time.Time
}

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


func renderTemplate(fs fs.FS, name TemplateName, statusCode int, wr http.ResponseWriter, data interface{}) {
	wr.Header().Set("Content-Type", "text/html; charset=utf-8")
	wr.WriteHeader(statusCode)

	tpl := template.New(fmt.Sprintf("%s.gohtml", name))
	tpl.Funcs(map[string]interface{}{
		"bytes": formatBytes,
	})
	template.Must(tpl.ParseFS(fs, fmt.Sprintf("%s.gohtml", name), "partials/*.gohtml"))
	if err := tpl.Execute(wr, data); err != nil {
		// TODO: We should probably send an error to the client
		log.Printf("Error rendering template: %v\n", err)
	}
}

func formatBytes(size int64) string {
	const multiple = 1024
	if size < multiple {
		return fmt.Sprintf("%d B", size)
	}

	denominator, power := int64(multiple), 0
	for n := size / multiple; n >= multiple; n /= multiple {
		denominator *= multiple
		power++
	}

	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(denominator), "KMGTPE"[power])
}
