package main

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type GitBackend struct {
	// mutex guards access to git commands. A read or write lock should be acquired in all exported methods,
	// and released at the end (via a deferral).
	mutex sync.RWMutex
	dir   string
	repo  *git.Repository
}

func NewGitBackend(dataDirectory string) (*GitBackend, error) {
	gitRepo, err := openOrInit(dataDirectory)
	if err != nil {
		return nil, fmt.Errorf("unable to open working directory: %w", err)
	}

	return &GitBackend{
		dir:  dataDirectory,
		repo: gitRepo,
	}, nil
}

func openOrInit(dataDirectory string) (*git.Repository, error) {
	gitRepo, err := git.PlainOpen(dataDirectory)
	if err == nil {
		return gitRepo, nil
	}
	gitRepo, err = git.PlainInit(dataDirectory, false)
	if err == nil {
		return gitRepo, nil
	}
	return nil, err
}

// walkFiles calls filepath.WalkDir, filtering out private data (.git and .wiki folders), and supplying
// both the path on disk and the web-appropriate path to the handler function.
func (g *GitBackend) walkFiles(handler func(filePath, webPath string, info fs.DirEntry) error) error {
	return filepath.WalkDir(g.dir, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == ".wiki" {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(g.dir, path)
		if err != nil {
			return err
		}
		return handler(path, strings.ReplaceAll(rel, string(filepath.Separator), "/"), info)
	})
}

func (g *GitBackend) resolveRevision(rv string) (*plumbing.Hash, error) {
	if rv == "" {
		rv = "HEAD"
	}
	return g.repo.ResolveRevision(plumbing.Revision(rv))
}

func (g *GitBackend) resolvePath(base, name string) (string, string, error) {
	p := filepath.Clean(filepath.Join(base, name))
	p = strings.ToLower(p)

	if strings.ContainsRune(p, '%') {
		return "", "", errors.New("paths cannot contain '%'")
	}

	rel, err := filepath.Rel(base, p)
	if err != nil || strings.HasPrefix(rel, ".") {
		return "", "", fmt.Errorf("attempt to escape directory")
	}

	parts := strings.Split(p, string(filepath.Separator))
	for i := range parts {
		if parts[i] == ".git" || parts[i] == ".wiki" {
			return "", "", fmt.Errorf("attempt to write to reserved directory")
		}
	}
	rel = strings.ReplaceAll(rel, string(filepath.Separator), "/")

	return p, rel, nil
}
