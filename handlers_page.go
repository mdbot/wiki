package main

import (
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"
)

type PageProvider interface {
	GetPage(title string) (*Page, error)
}

type RenderPageArgs struct {
	CommonPageArgs
	PageContent template.HTML
}

type ContentRenderer interface {
	Render([]byte) (string, error)
}

func ViewPageHandler(templateFs fs.FS, r ContentRenderer, pp PageProvider) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		pageTitle := strings.TrimPrefix(request.URL.Path, "/view/")

		page, err := pp.GetPage(pageTitle)
		if err != nil {
			http.NotFound(writer, request)
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


type DeletePageProvider interface {
	DeletePage(name string, message string, user string) error
}

type DeletePageArgs struct {
	CommonPageArgs
	PageName string
}

func DeletePageConfirmHandler(templateFs fs.FS) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		name := strings.TrimPrefix(request.URL.Path, "/delete/")
		renderTemplate(templateFs, DeletePage, http.StatusOK, writer, &DeletePageArgs{
			CommonPageArgs: CommonPageArgs{
				Session:   getSessionArgs(writer, request),
				PageTitle: "Delete Page",
			},
			PageName: name,
		})
	}
}

func DeletePageHandler(provider DeletePageProvider) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		name := strings.TrimPrefix(request.URL.Path, "/delete/")
		confirm := request.FormValue("confirm")
		if confirm == "" {
			http.Redirect(writer, request, "/delete/"+name, http.StatusTemporaryRedirect)
			return
		}
		message := request.FormValue("message")
		username := "Anonymoose"
		if user := getUserForRequest(request); user != nil {
			username = user.Name
		}
		err := provider.DeletePage(name, message, username)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.Redirect(writer, request, "/", http.StatusOK)
	}
}

type RenamePageProvider interface {
	RenamePage(name string, newName string, message string, user string) error
}

type RenamePageArgs struct {
	CommonPageArgs
	OldName string
}

func RenamePageConfirmHandler(templateFs fs.FS) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		name := strings.TrimPrefix(request.URL.Path, "/rename/")
		renderTemplate(templateFs, RenamePage, http.StatusOK, writer, &RenamePageArgs{
			CommonPageArgs: CommonPageArgs{
				Session:   getSessionArgs(writer, request),
				PageTitle: "Rename Page",
			},
			OldName: name,
		})
	}
}

func RenamePageHandler(provider RenamePageProvider) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		name := strings.TrimPrefix(request.URL.Path, "/rename/")
		newName := request.FormValue("newName")
		if newName == "" {
			http.Redirect(writer, request, "/rename/"+name, http.StatusTemporaryRedirect)
			return
		}
		message := request.FormValue("message")
		username := "Anonymoose"
		if user := getUserForRequest(request); user != nil {
			username = user.Name
		}
		err := provider.RenamePage(name, newName, message, username)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.Redirect(writer, request, "/"+newName, http.StatusOK)
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
