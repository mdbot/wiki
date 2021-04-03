package main

import (
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/mdbot/wiki/config"
)

type Templates struct {
	fs fs.FS
}

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

type ViewPageArgs struct {
	CommonPageArgs
	PageContent template.HTML
}

func (t *Templates) RenderPage(w http.ResponseWriter, r *http.Request, title, content string, log *LastModifiedDetails) {
	t.render("index.gohtml", http.StatusOK, w, &ViewPageArgs{
		CommonPageArgs: CommonPageArgs{
			Session:      getSessionArgs(w, r),
			PageTitle:    title,
			IsWikiPage:   true,
			LastModified: log,
		},
		PageContent: template.HTML(content),
	})
}

type EditPageArgs struct {
	CommonPageArgs
	PageContent string
}

func (t *Templates) RenderEditPage(w http.ResponseWriter, r *http.Request, title, content string) {
	t.render("edit.gohtml", http.StatusOK, w, &EditPageArgs{
		CommonPageArgs: CommonPageArgs{
			Session:   getSessionArgs(w, r),
			PageTitle: title,
		},
		PageContent: content,
	})
}

type DeletePageArgs struct {
	CommonPageArgs
	PageName string
}

func (t *Templates) RenderDeletePage(w http.ResponseWriter, r *http.Request, pageName string) {
	t.render("delete.gohtml", http.StatusOK, w, &DeletePageArgs{
		CommonPageArgs: CommonPageArgs{
			Session:   getSessionArgs(w, r),
			PageTitle: "Delete page",
		},
		PageName: pageName,
	})
}

type RenamePageArgs struct {
	CommonPageArgs
	OldName string
}

func (t *Templates) RenderRenamePage(w http.ResponseWriter, r *http.Request, oldName string) {
	t.render("rename.gohtml", http.StatusOK, w, &RenamePageArgs{
		CommonPageArgs: CommonPageArgs{
			Session:   getSessionArgs(w, r),
			PageTitle: "Rename page",
		},
		OldName: oldName,
	})
}

type ListPagesArgs struct {
	CommonPageArgs
	Pages []string
}

func (t *Templates) RenderPageList(w http.ResponseWriter, r *http.Request, pages []string) {
	t.render("list.gohtml", http.StatusOK, w, &ListPagesArgs{
		CommonPageArgs: CommonPageArgs{
			Session:   getSessionArgs(w, r),
			PageTitle: "Pages",
		},
		Pages: pages,
	})
}

type ListFilesArgs struct {
	CommonPageArgs
	Files []File
}

func (t *Templates) RenderFileList(w http.ResponseWriter, r *http.Request, files []File) {
	t.render("listfiles.gohtml", http.StatusOK, w, &ListFilesArgs{
		CommonPageArgs: CommonPageArgs{
			Session:   getSessionArgs(w, r),
			PageTitle: "Files",
		},
		Files: files,
	})
}

type UploadFileArgs struct {
	CommonPageArgs
}

func (t *Templates) RenderUploadForm(w http.ResponseWriter, r *http.Request) {
	t.render("upload.gohtml", http.StatusOK, w, &UploadFileArgs{
		CommonPageArgs: CommonPageArgs{
			Session:   getSessionArgs(w, r),
			PageTitle: "Upload file",
		},
	})
}

type HistoryPageArgs struct {
	CommonPageArgs
	History []*HistoryEntry
	Next    string
}

type HistoryEntry struct {
	Id      string
	User    string
	Time    time.Time
	Message string
}

func (t *Templates) RenderHistory(w http.ResponseWriter, r *http.Request, title string, entries []*HistoryEntry, next string) {
	t.render("history.gohtml", http.StatusOK, w, &HistoryPageArgs{
		CommonPageArgs: CommonPageArgs{
			Session:    getSessionArgs(w, r),
			PageTitle:  title,
			IsWikiPage: true,
		},
		History: entries,
		Next:    next,
	})
}

type ManageUsersArgs struct {
	CommonPageArgs
	Users []string
}

func (t *Templates) RenderManageUsers(w http.ResponseWriter, r *http.Request, users []string) {
	t.render("users.gohtml", http.StatusOK, w, &ManageUsersArgs{
		CommonPageArgs: CommonPageArgs{
			Session:   getSessionArgs(w, r),
			PageTitle: "Manage users",
		},
		Users: users,
	})
}

type ErrorPageArgs struct {
	CommonPageArgs
	ShowLoginForm bool
	OldPageTitle  string
}

func (t *Templates) RenderNotFound(w http.ResponseWriter, r *http.Request, isWiki bool, pageName string) {
	t.render("404.gohtml", http.StatusNotFound, w, &ErrorPageArgs{
		CommonPageArgs: CommonPageArgs{
			Session:      getSessionArgs(w, r),
			PageTitle:    "Page not found",
			IsWikiPage:   isWiki,
			IsError: true,
		},
		OldPageTitle: pageName,
	})
}

func (t *Templates) RenderUnauthorised(w http.ResponseWriter, r *http.Request) {
	t.render("error.gohtml", http.StatusUnauthorized, w, &ErrorPageArgs{
		CommonPageArgs: CommonPageArgs{
			Session:      getSessionArgs(w, r),
			PageTitle:    "Unauthorized",
			IsError: true,
		},
		ShowLoginForm: true,
	})
}

func (t *Templates) RenderForbidden(w http.ResponseWriter, r *http.Request) {
	t.render("error.gohtml", http.StatusForbidden, w, &ErrorPageArgs{
		CommonPageArgs: CommonPageArgs{
			Session:      getSessionArgs(w, r),
			PageTitle:    "Forbidden",
			IsError: true,
		},
	})
}

func (t *Templates) RenderInternalError(w http.ResponseWriter, r *http.Request) {
	t.render("error.gohtml", http.StatusInternalServerError, w, &ErrorPageArgs{
		CommonPageArgs: CommonPageArgs{
			Session:      getSessionArgs(w, r),
			PageTitle:    "Server Error",
			IsError: true,
		},
	})
}


func (t *Templates) render(name string, statusCode int, w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)

	tpl := template.New(name)
	tpl.Funcs(map[string]interface{}{
		"bytes": t.formatBytes,
	})
	template.Must(tpl.ParseFS(t.fs, name, "partials/*.gohtml"))
	if err := tpl.Execute(w, data); err != nil {
		// TODO: We should probably send an error to the client
		log.Printf("Error rendering template: %v\n", err)
	}
}

func (t *Templates) formatBytes(size int64) string {
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
