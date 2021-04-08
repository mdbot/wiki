package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
)

func (g *GitBackend) GetPage(title string) (*Page, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	filePath, gitPath, err := g.resolvePath(g.dir, fmt.Sprintf("%s.md", title))
	if err != nil {
		return nil, err
	}

	commitIter, err := g.repo.Log(&git.LogOptions{
		PathFilter: func(s string) bool {
			return s == gitPath
		},
	})
	if err != nil {
		return nil, err
	}
	commit, err := commitIter.Next()
	if err != nil {
		return nil, err
	}
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return &Page{
		Content: bytes,
		LastModified: &LogEntry{
			ChangeId: commit.Hash.String(),
			User:     commit.Author.Name,
			Time:     commit.Author.When,
			Message:  commit.Message,
		},
	}, nil
}

func (g *GitBackend) GetFile(name string) (io.ReadCloser, error) {
	filePath, _, err := g.resolvePath(g.dir, name)
	if err != nil {
		return nil, err
	}

	return os.Open(filePath)
}

func (g *GitBackend) GetConfig(name string) ([]byte, error) {
	filePath := filepath.Join(g.dir, ".wiki", fmt.Sprintf("%s.json.enc", name))
	return os.ReadFile(filePath)
}
