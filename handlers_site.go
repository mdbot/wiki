package main

import (
	"io"
	"log"
	"net/http"

	"github.com/mdbot/wiki/config"
)

func ViewSiteConfigHandler(t *Templates) http.HandlerFunc {
	return t.RenderViewSiteConfig
}

type SiteUpdater interface {
	Update(site *config.Site, responsible string) error
}

func UpdateSiteConfigHandler(updater SiteUpdater) http.HandlerFunc {
	fileBytes := func(request *http.Request, name string) ([]byte, error) {
		file, _, err := request.FormFile(name)
		if err != nil {
			if err == http.ErrMissingFile {
				return nil, nil
			}
			return nil, err
		}
		defer file.Close()
		return io.ReadAll(file)
	}

	return func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseMultipartForm(1 << 30); err != nil {
			log.Printf("Manage site: couldn't parse multipart data: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}

		siteName := request.FormValue("name")
		favicon, err := fileBytes(request, "favicon")
		if err != nil {
			log.Printf("Manage site: couldn't read favicon file: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}

		mainLogo, err := fileBytes(request, "logo")
		if err != nil {
			log.Printf("Manage site: couldn't read logo file: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}

		darkLogo, err := fileBytes(request, "darklogo")
		if err != nil {
			log.Printf("Manage site: couldn't read dark logo file: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}

		username := "Anonymoose"
		if user := getUserForRequest(request); user != nil {
			username = user.Name
		}

		if err := updater.Update(&config.Site{
			Name:     siteName,
			Favicon:  favicon,
			MainLogo: mainLogo,
			DarkLogo: darkLogo,
		}, username); err != nil {
			log.Printf("Manage site: unable to save new config: %v", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}

		writer.Header().Add("location", "/wiki/site")
		writer.WriteHeader(http.StatusSeeOther)
	}
}

func ServeFavicon(siteConfig *config.Site) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write(siteConfig.Favicon)
	}
}

func ServeMainLogo(siteConfig *config.Site) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write(siteConfig.MainLogo)
	}
}

func ServeDarkLogo(siteConfig *config.Site) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write(siteConfig.DarkLogo)
	}
}
