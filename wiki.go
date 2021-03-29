package main

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"os"
	"time"
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

func (g *GitPageProvider) GetPage(path string) (*Page, error) {
	path = path + ".md"
	commitIter, err := g.GitRepo.Log(&git.LogOptions{
		FileName: &path,
	})
	if err != nil {
		return nil, err
	}
	commit, err := commitIter.Next()
	if err != nil {
		return nil, err
	}
	bytes, err := os.ReadFile(g.GitDirectory + "/" + path)
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
	title = title + ".md"
	err := os.WriteFile(g.GitDirectory + "/" + title, content, os.FileMode(0644))
	if err != nil {
		return err
	}
	worktree, err := g.GitRepo.Worktree()
	if err != nil {
		return err
	}
	_, err = worktree.Add(title)
	if err != nil {
		return err
	}
	_, err = worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  user,
			Email: user+"@wiki",
			When:  time.Now(),
		},
	})
	if err != nil {
		return err
	}
	return nil
}
