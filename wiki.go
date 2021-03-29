package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type GitPageProvider struct {
	GitDirectory string
	GitRepo      *git.Repository
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

func (g *GitPageProvider) CreateDefaultMainPage() error {
	_, err := g.GetPage("MainPage")
	if err != nil {
		log.Printf("Creating default main page")
		return g.PutPage("MainPage", []byte("<h1>Welcome</h1>Welcome to the wiki."), "system", "Create welcome page")
	}
	return nil
}

func (g *GitPageProvider) GetPage(path string) (*Page, error) {
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

func (g *GitPageProvider) PutPage(title string, content []byte, user string, message string) error {
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
