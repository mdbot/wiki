package main

import (
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func (g *GitBackend) PageHistory(title string, start string, count int) (*History, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	_, gitPath, err := g.resolvePath(g.dir, fmt.Sprintf("%s.md", title))
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

func (g *GitBackend) GetPageAt(title, revision string) (*Page, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	_, gitPath, err := g.resolvePath(g.dir, fmt.Sprintf("%s.md", title))
	if err != nil {
		return nil, err
	}

	commit, b, err := g.pathAtRevision(gitPath, revision)
	if err != nil {
		return nil, err
	}

	return &Page{
		Content: b,
		LastModified: &LogEntry{
			ChangeId: commit.Hash.String(),
			User:     commit.Author.Name,
			Time:     commit.Author.When,
			Message:  commit.Message,
		},
	}, nil
}

// pathAtRevision gets the contents of the given path at the given revision, along the with commit object.
func (g *GitBackend) pathAtRevision(gitPath, revision string) (*object.Commit, []byte, error) {
	commitHash, err := g.resolveRevision(revision)
	if err != nil {
		return nil, nil, err
	}
	commit, err := g.repo.CommitObject(*commitHash)
	if err != nil {
		return nil, nil, err
	}
	file, err := commit.File(gitPath)
	if err != nil {
		return nil, nil, err
	}
	reader, err := file.Blob.Reader()
	if err != nil {
		return nil, nil, err
	}
	defer reader.Close()
	b, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, err
	}
	return commit, b, nil
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
