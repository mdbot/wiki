package main

import (
	"fmt"
	"github.com/mdbot/wiki/markdown"
	"io"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

type FileLister interface {
	ListFiles() ([]File, error)
}

func ListFilesHandler(t *Templates, fl FileLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, err := fl.ListFiles()
		if err != nil {
			log.Printf("Failed to list files: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		t.RenderFileList(w, r, files)
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

func UploadFormHandler(t *Templates) http.HandlerFunc {
	return t.RenderUploadForm
}

type FileProvider interface {
	GetFile(name string) (io.ReadCloser, error)
}

func FileHandler(provider FileProvider) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		name := strings.TrimPrefix(request.URL.Path, "/files/view/")
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
		if !markdown.CanEmbed(mimeType) {
			writer.Header().Add("Content-Disposition", "attachment")
		}
		_, _ = io.Copy(writer, reader)
	}
}

type DeleteFileProvider interface {
	DeleteFile(name string, message string, user string) error
}

func DeleteFileConfirmHandler(t *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/files/delete/")
		t.RenderDeleteFile(w, r, name)
	}
}

func DeleteFileHandler(provider DeleteFileProvider) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		name := strings.TrimPrefix(request.URL.Path, "/files/delete/")
		confirm := request.FormValue("confirm")
		if confirm == "" {
			http.Redirect(writer, request, "/files/delete/"+name, http.StatusSeeOther)
			return
		}
		message := request.FormValue("message")
		username := "Anonymoose"
		if user := getUserForRequest(request); user != nil {
			username = user.Name
		}
		err := provider.DeleteFile(name, message, username)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		putSessionKey(writer, request, sessionNoticeKey, fmt.Sprintf("Deleted file %s", name))
		http.Redirect(writer, request, "/", http.StatusSeeOther)
	}
}
