package main

import (
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

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
	canEmbed := func(mimeType string) bool {
		return strings.HasPrefix(mimeType, "image/") ||
			strings.HasPrefix(mimeType, "video/") ||
			strings.HasPrefix(mimeType, "audio/")
	}

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
