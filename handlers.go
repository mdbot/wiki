package main

import (
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	"github.com/greboid/wiki/config"
)

type TemplateName string

const (
	NotFound    TemplateName = "404"
	EditPage    TemplateName = "edit"
	HistoryPage TemplateName = "history"
	ViewPage    TemplateName = "index"
	ListPage    TemplateName = "list"
)

type CommonPageArgs struct {
	Session      SessionArgs
	PageTitle    string
	IsWikiPage   bool
	LastModified *LastModifiedDetails
}

type SessionArgs struct {
	CanEdit      bool
	Error        string
	User         *config.User
	CsrfField    template.HTML
	RequestedUrl string
}

type LastModifiedDetails struct {
	User string
	Time time.Time
}

type PageProvider interface {
	GetPage(title string) (*Page, error)
}

type RenderPageArgs struct {
	CommonPageArgs
	PageContent template.HTML
}

type NotFoundPageArgs struct {
	CommonPageArgs
}

type ContentRenderer interface {
	Render([]byte) (string, error)
}

func RenderPageHandler(templateFs fs.FS, r ContentRenderer, pp PageProvider) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		pageTitle := strings.TrimPrefix(request.URL.Path, "/view/")

		page, err := pp.GetPage(pageTitle)
		if err != nil {
			renderTemplate(templateFs, NotFound, http.StatusNotFound, writer, &NotFoundPageArgs{
				CommonPageArgs: CommonPageArgs{
					Session:    getSessionArgs(writer, request),
					PageTitle:  pageTitle,
					IsWikiPage: true,
				},
			})
			return
		}

		content, err := r.Render(page.Content)
		if err != nil {
			log.Printf("Failed to render markdown: %v\n", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}

		renderTemplate(templateFs, ViewPage, http.StatusOK, writer, &RenderPageArgs{
			CommonPageArgs: CommonPageArgs{
				Session:    getSessionArgs(writer, request),
				PageTitle:  pageTitle,
				IsWikiPage: true,
				LastModified: &LastModifiedDetails{
					User: page.LastModified.User,
					Time: page.LastModified.Time,
				},
			},

			PageContent: template.HTML(content),
		})
	}
}

type HistoryProvider interface {
	PageHistory(path string, start string, end int) (*History, error)
}

type HistoryEntry struct {
	Id      string
	User    string
	Time    time.Time
	Message string
}

type HistoryPageArgs struct {
	CommonPageArgs
	History []*HistoryEntry
	Next    string
}

func PageHistoryHandler(templateFs fs.FS, pp HistoryProvider) http.HandlerFunc {
	const historySize = 20

	return func(writer http.ResponseWriter, request *http.Request) {
		pageTitle := strings.TrimPrefix(request.URL.Path, "/history/")

		var start string
		var number = historySize + 1

		q := request.URL.Query()["after"]
		if q != nil {
			// If the user is paginating, request 22 items so we get the start item, the 20 we want to show, then
			// an extra one to tell if there's a next page or not.
			start = q[0]
			number = historySize + 2
		}

		history, err := pp.PageHistory(pageTitle, start, number)
		if err != nil {
			renderTemplate(templateFs, NotFound, http.StatusNotFound, writer, &NotFoundPageArgs{
				CommonPageArgs: CommonPageArgs{
					Session:    getSessionArgs(writer, request),
					PageTitle:  pageTitle,
					IsWikiPage: true,
				},
			})
			return
		}

		var entries []*HistoryEntry
		for i := range history.Entries {
			c := history.Entries[i]
			if c.ChangeId == start {
				continue
			}
			entries = append(entries, &HistoryEntry{
				Id:      c.ChangeId,
				User:    c.User,
				Time:    c.Time,
				Message: c.Message,
			})
			if len(entries) == historySize {
				break
			}
		}

		var next string
		if len(history.Entries) == number {
			next = history.Entries[number-2].ChangeId
		}

		renderTemplate(templateFs, HistoryPage, http.StatusOK, writer, &HistoryPageArgs{
			CommonPageArgs: CommonPageArgs{
				Session:    getSessionArgs(writer, request),
				PageTitle:  pageTitle,
				IsWikiPage: true,
			},

			History: entries,
			Next:    next,
		})
	}
}

type EditPageArgs struct {
	CommonPageArgs
	PageContent string
}

func EditPageHandler(templateFs fs.FS, pp PageProvider) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		pageTitle := strings.TrimPrefix(request.URL.Path, "/edit/")

		var content string
		if page, err := pp.GetPage(pageTitle); err == nil {
			content = string(page.Content)
		}

		renderTemplate(templateFs, EditPage, http.StatusOK, writer, &EditPageArgs{
			CommonPageArgs: CommonPageArgs{
				Session:   getSessionArgs(writer, request),
				PageTitle: pageTitle,
			},
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
		username := "Anonymoose"
		if user := getUserForRequest(request); user != nil {
			username = user.Name
		}

		if err := pe.PutPage(pageTitle, []byte(content), username, message); err != nil {
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
	CommonPageArgs
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
			CommonPageArgs: CommonPageArgs{
				Session:   getSessionArgs(writer, request),
				PageTitle: "Index",
			},
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

type Authenticator interface {
	Authenticate(username, password string) (*config.User, error)
}

func LoginHandler(store sessions.Store, auth Authenticator) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		username := request.FormValue("username")
		password := request.FormValue("password")
		redirect := request.FormValue("redirect")

		// Only allow relative redirects
		if !strings.HasPrefix(redirect, "/") || strings.HasPrefix(redirect, "//") {
			redirect = "/"
		}

		user, err := auth.Authenticate(username, password)
		if err != nil {
			putSessionKey(store, writer, request, sessionErrorKey, fmt.Sprintf("Failed to login: %v", err))
			writer.Header().Set("location", redirect)
			writer.WriteHeader(http.StatusSeeOther)
		} else {
			putSessionKey(store, writer, request, sessionUserKey, user.Name)
			writer.Header().Set("location", redirect)
			writer.WriteHeader(http.StatusSeeOther)
		}
	}
}

func renderTemplate(fs fs.FS, name TemplateName, statusCode int, wr http.ResponseWriter, data interface{}) {
	wr.Header().Set("Content-Type", "text/html; charset=utf-8")
	wr.WriteHeader(statusCode)

	tpl := template.Must(template.ParseFS(fs, fmt.Sprintf("%s.gohtml", name), "partials/*.gohtml"))
	if err := tpl.Execute(wr, data); err != nil {
		// TODO: We should probably send an error to the client
		log.Printf("Error rendering template: %v\n", err)
	}
}
