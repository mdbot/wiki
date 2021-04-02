package main

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/mdbot/wiki/config"
)

type TemplateName string

const (
	NotFound      TemplateName = "404"
	Unauthorized  TemplateName = "error"
	Forbidden     TemplateName = "error"
	ServerError   TemplateName = "error"
	EditPage      TemplateName = "edit"
	HistoryPage   TemplateName = "history"
	ViewPage      TemplateName = "index"
	ListPage      TemplateName = "list"
	ListFilesPage TemplateName = "listfiles"
	UploadPage    TemplateName = "upload"
	RenamePage    TemplateName = "rename"
	DeletePage    TemplateName = "delete"
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

type ErrorPageArgs struct {
	CommonPageArgs
	ShowLoginForm bool
}

type RenamePageArgs struct {
	CommonPageArgs
	OldName string
}

type DeletePageArgs struct {
	CommonPageArgs
	PageName string
}

type ContentRenderer interface {
	Render([]byte) (string, error)
}

func RenderPageHandler(templateFs fs.FS, r ContentRenderer, pp PageProvider) http.HandlerFunc {
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
			http.NotFound(writer, request)
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

type FileLister interface {
	ListFiles() ([]File, error)
}

type ListFilesArgs struct {
	CommonPageArgs
	Files []File
}

func ListFilesHandler(templateFs fs.FS, fl FileLister) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		files, err := fl.ListFiles()
		if err != nil {
			log.Printf("Failed to list files: %v\n", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}

		renderTemplate(templateFs, ListFilesPage, http.StatusOK, writer, &ListFilesArgs{
			CommonPageArgs: CommonPageArgs{
				Session:   getSessionArgs(writer, request),
				PageTitle: "Files",
			},
			Files: files,
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

func LoginHandler(auth Authenticator) http.HandlerFunc {
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
			putSessionKey(writer, request, sessionErrorKey, fmt.Sprintf("Failed to login: %v", err))
		} else {
			putSessionKey(writer, request, sessionUserKey, user.Name)
		}
		writer.Header().Set("location", redirect)
		writer.WriteHeader(http.StatusSeeOther)
	}
}

func LogoutHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		redirect := request.FormValue("redirect")

		// Only allow relative redirects
		if !strings.HasPrefix(redirect, "/") || strings.HasPrefix(redirect, "//") {
			redirect = "/"
		}

		clearSessionKey(writer, request, sessionUserKey)
		writer.Header().Set("location", redirect)
		writer.WriteHeader(http.StatusSeeOther)
	}
}

type FileStore interface {
	PutFile(name string, content io.ReadCloser, user string, message string) error
}

func UploadHandler(store FileStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseMultipartForm(1 << 30); err != nil {
			log.Printf("Upload failed: couldn't parse multipart data: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}

		file, _, err := request.FormFile("file")
		if err != nil {
			log.Printf("Upload failed: couldn't read file: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}

		name := request.FormValue("name")
		if name == "" || !strings.ContainsRune(name, '.') {
			log.Printf("Upload failed: invalid file name specified: %v", name)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}

		message := request.FormValue("message")
		username := "Anonymoose"
		if user := getUserForRequest(request); user != nil {
			username = user.Name
		}

		if err := store.PutFile(name, file, username, message); err != nil {
			log.Printf("Upload failed: couldn't save file: %v", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}

		writer.WriteHeader(http.StatusNoContent)
	}
}

type UploadPageArgs struct {
	CommonPageArgs
}

func UploadFormHandler(templateFs fs.FS) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		renderTemplate(templateFs, UploadPage, http.StatusOK, writer, &UploadPageArgs{
			CommonPageArgs: CommonPageArgs{
				Session:   getSessionArgs(writer, request),
				PageTitle: "Upload file",
			},
		})
	}
}

type FileProvider interface {
	GetFile(name string) (io.ReadCloser, error)
}

func FileHandler(provider FileProvider) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		name := strings.TrimPrefix(request.URL.Path, "/file/")
		reader, err := provider.GetFile(name)
		if err != nil {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		defer reader.Close()

		mimeType := mime.TypeByExtension(filepath.Ext(name))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}

		writer.Header().Add("Content-Type", mimeType)
		writer.Header().Add("X-Content-Type-Options", "nosniff")
		if !canEmbed(mimeType) {
			writer.Header().Add("Content-Disposition", "attachment")
		}
		_, _ = io.Copy(writer, reader)
	}
}

type DeletePageProvider interface {
	DeletePage(name string, message string, user string) error
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

func canEmbed(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/") ||
		strings.HasPrefix(mimeType, "video/") ||
		strings.HasPrefix(mimeType, "audio/")
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
