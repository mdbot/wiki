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
)

type TemplateName string

const (
	NotFound TemplateName = "404"
	EditPage TemplateName = "edit"
	ViewPage TemplateName = "index"
	ListPage TemplateName = "list"
)

type CommonPageArgs struct {
	Session      SessionArgs
	PageTitle    string
	IsWikiPage   bool
	LastModified *LastModifiedDetails
}

type SessionArgs struct {
	CanEdit bool
	Error   string
	User    *User
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
					Session:    getSessionArgs(request),
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
				Session:    getSessionArgs(request),
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
				Session:   getSessionArgs(request),
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
				Session:   getSessionArgs(request),
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
	Authenticate(username, password string) (*User, error)
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
			putSession(store, writer, request, sessionErrorKey, fmt.Sprintf("Failed to login: %v", err))
			writer.Header().Set("location", redirect)
			writer.WriteHeader(http.StatusSeeOther)
		} else {
			putSession(store, writer, request, sessionUserKey, user.Name)
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
