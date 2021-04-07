package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
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

func (g *GitBackend) PageExists(title string) bool {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	filePath, _, err := resolvePath(g.dir, fmt.Sprintf("%s.md", title))
	if err != nil {
		return false
	}

	fi, err := os.Stat(filePath)
	return err == nil && !fi.IsDir()
}

func (g *GitBackend) PageHistory(title string, start string, count int) (*History, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	_, gitPath, err := resolvePath(g.dir, fmt.Sprintf("%s.md", title))
	if err != nil {
		return nil, err
	}

	revision, err := g.resolveRevision(start)
	if err != nil {
		return nil, err
	}

	commitIter, err := g.repo.Log(&git.LogOptions{
		From: *revision,
		PathFilter: func(s string) bool {
			return s == gitPath
		},
	})
	if err != nil {
		return nil, err
	}

	var history []*LogEntry
	for i := 0; i < count; i++ {
		commit, err := commitIter.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		history = append(history, &LogEntry{
			ChangeId: commit.Hash.String(),
			User:     commit.Author.Name,
			Time:     commit.Author.When,
			Message:  commit.Message,
		})
	}

	return &History{Entries: history}, nil
}

func (g *GitBackend) GetPage(title string) (*Page, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	filePath, gitPath, err := resolvePath(g.dir, fmt.Sprintf("%s.md", title))
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
	filePath, _, err := resolvePath(g.dir, name)
	if err != nil {
		return nil, err
	}

	return os.Open(filePath)
}

func (g *GitBackend) GetConfig(name string) ([]byte, error) {
	filePath := filepath.Join(g.dir, ".wiki", fmt.Sprintf("%s.json.enc", name))
	return os.ReadFile(filePath)
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

func (g *GitBackend) PutPage(title string, content []byte, user string, message string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	filePath, gitPath, err := resolvePath(g.dir, fmt.Sprintf("%s.md", title))
	if err != nil {
		return err
	}

	return g.writeFile(filePath, gitPath, bytes.NewReader(content), user, message)
}

func (g *GitBackend) PutFile(name string, content io.ReadCloser, user string, message string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	defer content.Close()

	filePath, gitPath, err := resolvePath(g.dir, name)
	if err != nil {
		return err
	}

	return g.writeFile(filePath, gitPath, content, user, message)
}

func (g *GitBackend) PutConfig(name string, content []byte, user string, message string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	filePath := filepath.Join(g.dir, ".wiki", fmt.Sprintf("%s.json.enc", name))
	gitPath := filepath.Join(".wiki", fmt.Sprintf("%s.json.enc", name))

	return g.writeFile(filePath, gitPath, bytes.NewReader(content), user, message)
}

func (g *GitBackend) writeFile(filePath, gitPath string, content io.Reader, user, message string) error {
	if err := os.MkdirAll(filepath.Dir(filePath), os.FileMode(0755)); err != nil {
		return err
	}

	f, err := os.Create(filePath)
	if err != nil {
		return err
	}

	if _, err := io.Copy(f, content); err != nil {
		_ = f.Close()
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	worktree, err := g.repo.Worktree()
	if err != nil {
		return err
	}

	if _, err := worktree.Add(gitPath); err != nil {
		return err
	}

	_, err = worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  user,
			Email: user + "@wiki",
			When:  time.Now(),
		},
	})
	return err
}

func (g *GitBackend) RenamePage(name string, newName string, message string, user string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	_, gitPath, err := resolvePath(g.dir, fmt.Sprintf("%s.md", name))
	if err != nil {
		log.Printf("Unable to resolve old path: %s -> %s: %s", name, newName, err.Error())
		return err
	}
	_, newGitPath, err := resolvePath(g.dir, fmt.Sprintf("%s.md", newName))
	if err != nil {
		log.Printf("Unable to resolve new path: %s -> %s: %s", name, newName, err.Error())
		return err
	}
	worktree, err := g.repo.Worktree()
	if err != nil {
		log.Printf("Unable to get worktree: %s", err.Error())
		return err
	}
	_, err = worktree.Move(gitPath, newGitPath)
	if err != nil {
		log.Printf("Unable to rename git: %s -> %s: %s", name, newName, err.Error())
		return err
	}
	_, err = worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  user,
			Email: user + "@wiki",
			When:  time.Now(),
		},
	})
	return nil
}

func (g *GitBackend) DeletePage(name string, message string, user string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	return g.delete(fmt.Sprintf("%s.md", name), message, user)
}

func (g *GitBackend) DeleteFile(name string, message string, user string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	return g.delete(name, message, user)
}

func (g *GitBackend) delete(name, message, user string) error {
	_, gitPath, err := resolvePath(g.dir, name)
	if err != nil {
		return err
	}
	worktree, err := g.repo.Worktree()
	if err != nil {
		return err
	}
	_, err = worktree.Remove(gitPath)
	if err != nil {
		return err
	}
	_, err = worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  user,
			Email: user + "@wiki",
			When:  time.Now(),
		},
	})
	return nil
}

func (g *GitBackend) RecentChanges(start string, count int) ([]*RecentChange, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	revision, err := g.resolveRevision(start)
	if err != nil {
		return nil, err
	}

	commitIter, err := g.repo.Log(&git.LogOptions{From: *revision})
	if err != nil {
		return nil, err
	}

	var history []*RecentChange
	for i := 0; i < count; i++ {
		commit, err := commitIter.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		stats, err := commit.Stats()
		if err != nil {
			return nil, err
		}

		entry := &RecentChange{
			LogEntry: LogEntry{
				ChangeId: commit.Hash.String(),
				User:     commit.Author.Name,
				Time:     commit.Author.When,
				Message:  commit.Message,
			},
		}

		for j := range stats {
			file := stats[j]
			if filepath.Dir(file.Name) == ".wiki" {
				entry.Config = strings.TrimSuffix(filepath.Base(file.Name), ".json.enc")
				break
			} else if path.Ext(file.Name) == ".md" {
				entry.Page = strings.TrimSuffix(file.Name, ".md")
				break
			} else {
				entry.File = file.Name
				break
			}
		}

		history = append(history, entry)
	}
	return history, nil
}

func (g *GitBackend) SearchWiki(pattern string) []SearchResult {
	trimPrefix := filepath.Clean(g.dir) + string(filepath.Separator)
	results := searchDirectory(g.dir, pattern)
	var output []SearchResult
	for index := range results {
		output = append(output, SearchResult{
			Filename:   strings.TrimSuffix(strings.TrimPrefix(results[index].Filename, trimPrefix), ".md"),
			FoundLines: results[index].FoundLines,
		})
	}
	return output
}

func (g *GitBackend) resolveRevision(rv string) (*plumbing.Hash, error) {
	if rv == "" {
		rv = "HEAD"
	}
	return g.repo.ResolveRevision(plumbing.Revision(rv))
}

func resolvePath(base, name string) (string, string, error) {
	p := filepath.Clean(filepath.Join(base, name))
	p = strings.ToLower(p)

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
