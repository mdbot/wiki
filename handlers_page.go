package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

type PageProvider interface {
	GetPage(title string) (*Page, error)
}

type ContentRenderer interface {
	Render([]byte) (string, error)
}

func ViewPageHandler(t *Templates, renderer ContentRenderer, pp PageProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pageTitle := strings.TrimPrefix(r.URL.Path, "/view/")

		page, err := pp.GetPage(pageTitle)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		content, err := renderer.Render(page.Content)
		if err != nil {
			log.Printf("Failed to render markdown: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		t.RenderPage(w, r, pageTitle, content, &LastModifiedDetails{
			User: page.LastModified.User,
			Time: page.LastModified.Time,
		})
	}
}

func EditPageHandler(t *Templates, pp PageProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pageTitle := strings.TrimPrefix(r.URL.Path, "/edit/")

		var content string
		if page, err := pp.GetPage(pageTitle); err == nil {
			content = string(page.Content)
		}

		t.RenderEditPage(w, r, pageTitle, content)
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

func DeletePageConfirmHandler(t *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/delete/")
		t.RenderDeletePage(w, r, name)
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

func RenamePageConfirmHandler(t  *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/rename/")
		t.RenderRenamePage(w, r, name)
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
		http.Redirect(writer, request, "/view/"+newName, http.StatusTemporaryRedirect)
	}
}

type PageLister interface {
	ListPages() ([]string, error)
}

func ListPagesHandler(t *Templates, pl PageLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pages, err := pl.ListPages()
		if err != nil {
			log.Printf("Failed to list pages: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		t.RenderPageList(w, r, pages)
	}
}
