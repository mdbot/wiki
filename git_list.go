package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func (g *GitBackend) PageExists(title string) bool {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	filePath, _, err := g.resolvePath(g.dir, fmt.Sprintf("%s.md", title))
	if err != nil {
		return false
	}

	fi, err := os.Stat(filePath)
	return err == nil && !fi.IsDir()
}

func (g *GitBackend) ListPages() ([]string, error) {
	var pages []string
	return pages, g.walkFiles(func(filePath, webPath string, info fs.DirEntry) error {
		if filepath.Ext(filePath) == ".md" {
			pages = append(pages, strings.TrimSuffix(webPath, ".md"))
		}
		return nil
	})
}

func (g *GitBackend) ListFiles() ([]File, error) {
	var files []File

	return files, g.walkFiles(func(filePath, webPath string, info fs.DirEntry) error {
		if filepath.Ext(filePath) != ".md" {
			stat, err := os.Stat(filePath)
			if err != nil {
				return err
			}

			files = append(files, File{
				Name: webPath,
				Size: stat.Size(),
			})
		}
		return nil
	})
}
