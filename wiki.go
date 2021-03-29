package main

import (
	"github.com/go-git/go-git/v5"
	"os"
)

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

func getPage(repo *git.Repository, path string) (*Page, error) {
	commitIter, err := repo.Log(&git.LogOptions{
		FileName: &path,
	})
	if err != nil {
		return nil, err
	}
	commit, err := commitIter.Next()
	if err != nil {
		return nil, err
	}
	bytes, err := os.ReadFile(*workDir+"/"+path)
	if err != nil {
		return nil, err
	}
	return &Page{
		Title:        path,
		Content:      string(bytes),
		LastModified: &LogEntry{
			ChangeId: commit.Hash.String(),
			User:     commit.Author.Name,
			Time:     commit.Author.When,
			Message:  commit.Message,
		},
	}, nil
}