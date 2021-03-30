package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type GitBackend struct {
	GitDirectory string
	GitRepo      *git.Repository
}

func NewGitBackend(dataDirectory string) (*GitBackend, error) {
	gitRepo, err := openOrInit(dataDirectory)
	if err != nil {
		return nil, fmt.Errorf("unable to open working directory: %w", err)
	}

	return &GitBackend{
		GitDirectory: dataDirectory,
		GitRepo:      gitRepo,
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

func (g *GitBackend) CreateDefaultMainPage() error {
	_, err := g.GetPage("MainPage")
	if err != nil {
		log.Printf("Creating default main page")
		return g.PutPage("MainPage", []byte("# Welcome\r\n\r\nWelcome to the wiki."), "system", "Create welcome page")
	}
	return nil
}

func (g *GitBackend) GetPage(path string) (*Page, error) {
	filePath, gitPath, err := resolvePath(g.GitDirectory, path)
	if err != nil {
		return nil, err
	}

	commitIter, err := g.GitRepo.Log(&git.LogOptions{
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
		Content: string(bytes),
		LastModified: &LogEntry{
			ChangeId: commit.Hash.String(),
			User:     commit.Author.Name,
			Time:     commit.Author.When,
			Message:  commit.Message,
		},
	}, nil
}

func (g *GitBackend) ListPages() ([]string, error) {
	pages, err := g.listPages(g.GitDirectory, "")
	if err != nil {
		return nil, err
	}
	sort.Strings(pages)
	return pages, nil
}

// listPages recursively finds pages within the given directory. The prefix is prepended to each returned path.
func (g *GitBackend) listPages(dir string, prefix string) ([]string, error) {
	var res []string

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for i := range files {
		if files[i].IsDir() {
			files, err := g.listPages(
				filepath.Join(dir, files[i].Name()),
				filepath.Join(prefix, files[i].Name()),
			)
			if err != nil {
				return nil, err
			}
			res = append(res, files...)
		} else if filepath.Ext(files[i].Name()) == ".md" {
			res = append(res, strings.TrimSuffix(filepath.Join(prefix, files[i].Name()), ".md"))
		}
	}

	return res, nil
}

func (g *GitBackend) PutPage(title string, content []byte, user string, message string) error {
	filePath, gitPath, err := resolvePath(g.GitDirectory, title)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(filePath), os.FileMode(0644)); err != nil {
		return err
	}

	if err := os.WriteFile(filePath, content, os.FileMode(0644)); err != nil {
		return err
	}
	worktree, err := g.GitRepo.Worktree()
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

func resolvePath(base, title string) (string, string, error) {
	p := filepath.Clean(filepath.Join(base, fmt.Sprintf("%s.md", title)))
	p = strings.ToLower(p)

	rel, err := filepath.Rel(base, p)
	if err != nil || strings.HasPrefix(rel, ".") {
		return "", "", fmt.Errorf("attempt to escape directory")
	}

	parts := strings.Split(p, string(filepath.Separator))
	for i := range parts {
		if strings.EqualFold(".git", parts[i]) {
			return "", "", fmt.Errorf("git directories cannot be written to")
		}
	}

	return p, rel, nil
}
