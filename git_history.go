package main

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/sergi/go-diff/diffmatchpatch"
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

func (g *GitBackend) RevertPage(title, revision, user, message string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	filePath, gitPath, err := g.resolvePath(g.dir, fmt.Sprintf("%s.md", title))
	if err != nil {
		return err
	}

	_, b, err := g.pathAtRevision(gitPath, revision)
	if err != nil {
		return err
	}

	return g.writeFile(filePath, gitPath, bytes.NewReader(b), user, message)
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

		myTree, err := commit.Tree()
		if err != nil {
			return nil, err
		}

		parent, err := commit.Parent(0)
		if err != nil {
			return nil, err
		}

		parentTree, err := parent.Tree()
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

		addFile := func(name string) {
			if filepath.Dir(name) == ".wiki" {
				entry.Config = strings.TrimSuffix(filepath.Base(name), ".json.enc")
			} else if path.Ext(name) == ".md" {
				entry.Page = strings.TrimSuffix(name, ".md")
			} else {
				entry.File = name
			}
		}

		var hashes = make(map[string]plumbing.Hash)
		if err := g.walkTreeFiles(myTree, "", func(name string, entry object.TreeEntry) error {
			hashes[name] = entry.Hash
			return nil
		}); err != nil {
			return nil, err
		}

		// Find any hashes that have changed, or files that exist in the parent tree that no longer do
		if err := g.walkTreeFiles(parentTree, "", func(name string, entry object.TreeEntry) error {
			if hash := hashes[name]; hash != entry.Hash {
				addFile(name)
			}
			delete(hashes, name)
			return nil
		}); err != nil {
			return nil, err
		}

		// Any leftover files are new compared to the parent tree
		for j := range hashes {
			addFile(j)
		}

		history = append(history, entry)
	}
	return history, nil
}

func (g *GitBackend) walkTreeFiles(tree *object.Tree, prefix string, h func(name string, entry object.TreeEntry) error) error {
	for i := range tree.Entries {
		entry := tree.Entries[i]
		if entry.Mode.IsFile() {
			if err := h(path.Join(prefix, entry.Name), entry); err != nil {
				return err
			}
		} else {
			subtree, err := tree.Tree(entry.Name)
			if err != nil {
				return err
			}
			if err := g.walkTreeFiles(subtree, path.Join(prefix, entry.Name), h); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *GitBackend) PathDiff(path string, startRevision string, endRevision string) (string, error) {
	_, gitPath, err := g.resolvePath(g.dir, fmt.Sprintf("%s.md", path))
	_, startContent, err := g.pathAtRevision(gitPath, startRevision)
	if err != nil {
		return "", err
	}
	_, endContent, err := g.pathAtRevision(gitPath, endRevision)
	if err != nil {
		return "", err
	}
	thing := diffmatchpatch.New()
	diffs := thing.DiffMain(string(startContent), string(endContent), true)
	pretty := thing.DiffPrettyHtml(diffs)
	return pretty, nil
}
