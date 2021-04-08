package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func (g *GitBackend) PutPage(title string, content []byte, user string, message string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	filePath, gitPath, err := g.resolvePath(g.dir, fmt.Sprintf("%s.md", title))
	if err != nil {
		return err
	}

	return g.writeFile(filePath, gitPath, bytes.NewReader(content), user, message)
}

func (g *GitBackend) PutFile(name string, content io.ReadCloser, user string, message string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	defer content.Close()

	filePath, gitPath, err := g.resolvePath(g.dir, name)
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

	_, gitPath, err := g.resolvePath(g.dir, fmt.Sprintf("%s.md", name))
	if err != nil {
		log.Printf("Unable to resolve old path: %s -> %s: %s", name, newName, err.Error())
		return err
	}
	_, newGitPath, err := g.resolvePath(g.dir, fmt.Sprintf("%s.md", newName))
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
	_, gitPath, err := g.resolvePath(g.dir, name)
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
