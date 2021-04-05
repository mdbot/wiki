package config

import (
	"fmt"
	"net/http"
	"strings"
)

const siteSettingsName = "site"

type Site struct {
	Name     string
	Favicon  []byte
	MainLogo []byte
	DarkLogo []byte

	store Store
}

func LoadSite(store Store) (*Site, error) {
	s := &Site{
		store: store,
	}

	if err := store.GetSettings(siteSettingsName, &s); err != nil {
		return nil, err
	}

	dirty := false

	if s.Name == "" {
		s.Name = "Wiki"
		dirty = true
	}

	if dirty {
		_ = store.PutSettings(siteSettingsName, "System", "Initialising site config", s)
	}

	return s, nil
}

func (s *Site) Update(config *Site, responsible string) error {
	if config.Name != "" {
		s.Name = config.Name
	}
	if config.Favicon != nil {
		if !strings.HasPrefix(http.DetectContentType(config.Favicon), "image/") {
			return fmt.Errorf("favicon is not an image")
		}

		s.Favicon = config.Favicon
	}
	if config.MainLogo != nil {
		if !strings.HasPrefix(http.DetectContentType(config.MainLogo), "image/") {
			return fmt.Errorf("main logo is not an image")
		}

		s.MainLogo = config.MainLogo
	}
	if config.DarkLogo != nil {
		if !strings.HasPrefix(http.DetectContentType(config.MainLogo), "image/") {
			return fmt.Errorf("dark logo is not an image")
		}

		s.DarkLogo = config.DarkLogo
	}

	return s.store.PutSettings(siteSettingsName, responsible, "Updating site config", s)
}
