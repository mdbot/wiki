package main

import (
	"github.com/go-git/go-git/v5"
	"os"
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
