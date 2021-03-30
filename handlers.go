package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	wikilink "github.com/13rac1/goldmark-wikilink"
	"github.com/gorilla/handlers"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

type TemplateName string
const (
	NotFound TemplateName = "404"
	EditPage TemplateName = "edit"
	ViewPage TemplateName = "index"
	ListPage TemplateName = "list"
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

type NotFoundPageArgs struct {
	PageTitle  string
	IsWikiPage bool
	CanEdit    bool
}

func NotFoundHandler(h http.Handler, templateFs fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fakeWriter := &notFoundInterceptWriter{realWriter: w}

		h.ServeHTTP(fakeWriter, r)

		if fakeWriter.status == http.StatusNotFound {
			renderTemplate(templateFs, NotFound, http.StatusNotFound, w, &NotFoundPageArgs{
				PageTitle:  "Page not found",
				IsWikiPage: false,
				CanEdit:    false,
			})
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

type FileNameNormalizer struct{}

func (_ FileNameNormalizer) Normalize(linkText string) string {
	return url.PathEscape(linkText)
}

type LastModifiedDetails struct {
	User string
	Time time.Time
}

type RenderPageArgs struct {
	PageTitle    string
	PageContent  template.HTML
	CanEdit      bool
	IsWikiPage   bool
	LastModified LastModifiedDetails
}

func RenderPageHandler(templateFs fs.FS, pp PageProvider) http.HandlerFunc {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle(*codeStyle),
			),
			wikilink.New(wikilink.WithFilenameNormalizer(FileNameNormalizer{})),
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)

	return func(writer http.ResponseWriter, request *http.Request) {
		pageTitle := strings.TrimPrefix(request.URL.Path, "/view/")

		page, err := pp.GetPage(pageTitle)
		if err != nil {
			renderTemplate(templateFs, NotFound, http.StatusNotFound, writer, &RenderPageArgs{
				PageTitle:  pageTitle,
				CanEdit:    true,
				IsWikiPage: true,
			})
			return
		}

		b := &bytes.Buffer{}
		if err := md.Convert([]byte(page.Content), b); err != nil {
			log.Printf("Failed to render markdown: %v\n", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}

		renderTemplate(templateFs, ViewPage, http.StatusOK, writer, &RenderPageArgs{
			PageTitle:   pageTitle,
			CanEdit:     true,
			IsWikiPage:  true,
			PageContent: template.HTML(b.String()),
			LastModified: LastModifiedDetails{
				User: page.LastModified.User,
				Time: page.LastModified.Time,
			},
		})
	}
}

type EditPageArgs struct {
	PageTitle   string
	PageContent string
	CanEdit     bool
}

func EditPageHandler(templateFs fs.FS, pp PageProvider) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		pageTitle := strings.TrimPrefix(request.URL.Path, "/edit/")

		var content string
		if page, err := pp.GetPage(pageTitle); err == nil {
			content = page.Content
		}

		renderTemplate(templateFs, EditPage, http.StatusOK, writer, &EditPageArgs{
			PageTitle:   pageTitle,
			CanEdit:     true,
			PageContent: content,
		})
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

type PageLister interface {
	ListPages() ([]string, error)
}

type ListPagesArgs struct {
	Pages []string
}

func ListPagesHandler(templateFs fs.FS, pl PageLister) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		pages, err := pl.ListPages()
		if err != nil {
			log.Printf("Failed to list pages: %v\n", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}

		renderTemplate(templateFs, ListPage, http.StatusOK, writer, &ListPagesArgs{
			Pages: pages,
		})
	}
}

func RedirectMainPageHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Add("Location", fmt.Sprintf("/view/%s", *mainPage))
		writer.WriteHeader(http.StatusSeeOther)
	}
}

func renderTemplate(fs fs.FS, name TemplateName, statusCode int, wr http.ResponseWriter, data interface{}) {
	wr.Header().Set("Content-Type", "text/html; charset=utf-8")
	wr.WriteHeader(statusCode)

	tpl := template.Must(template.ParseFS(fs, fmt.Sprintf("%s.gohtml", name)))
	if err := tpl.Execute(wr, data); err != nil {
		// TODO: We should probably send an error to the client
		log.Printf("Error rendering template: %v\n", err)
	}
}
