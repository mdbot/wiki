package main

import (
	"bufio"
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type SearchResult struct {
	filename   string
	foundLines []string
}

func searchDirectory(path string, pattern string) []SearchResult {
	results := make([]SearchResult, 0)
	_ = filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() ||
			strings.HasPrefix(path, ".git") ||
			strings.HasPrefix(path, ".wiki") ||
			!strings.HasSuffix(path, ".md") {
			return nil
		}
		result, err := searchFile(path, pattern)
		if err == nil {
			results = append(results, result)
		}
		return nil
	})
	return results
}

func searchFile(file string, pattern string) (SearchResult, error) {
	f, err := os.Open(file)
	if err != nil {
		return SearchResult{}, err
	}
	defer func() {
		_ = f.Close()
	}()
	result := SearchResult{
		filename:   file,
		foundLines: nil,
	}
	found := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if bytes.Contains(scanner.Bytes(), []byte(pattern)) {
			found = true
			result.foundLines = append(result.foundLines, scanner.Text())
		}
	}
	if found {
		return result, nil
	}
	return SearchResult{}, errors.New("no result")
}
