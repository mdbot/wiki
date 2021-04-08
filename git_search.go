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

func (g *GitBackend) SearchWiki(pattern string) []SearchResult {
	trimPrefix := filepath.Clean(g.dir) + string(filepath.Separator)
	patternBytes := bytes.ToLower([]byte(pattern))
	results := searchDirectory(g.dir, patternBytes)
	var output []SearchResult
	for index := range results {
		output = append(output, SearchResult{
			Filename:   strings.TrimSuffix(strings.TrimPrefix(results[index].Filename, trimPrefix), ".md"),
			FoundLines: results[index].FoundLines,
		})
	}
	return output
}

type SearchResult struct {
	Filename   string
	FoundLines []string
}

func searchDirectory(path string, pattern []byte) []SearchResult {
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

func searchFile(file string, pattern []byte) (SearchResult, error) {
	f, err := os.Open(file)
	if err != nil {
		return SearchResult{}, err
	}
	defer func() {
		_ = f.Close()
	}()
	result := SearchResult{
		Filename:   file,
		FoundLines: nil,
	}
	found := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if bytes.Contains(bytes.ToLower(scanner.Bytes()), pattern) {
			found = true
			result.FoundLines = append(result.FoundLines, scanner.Text())
		}
	}
	if found {
		return result, nil
	}
	return SearchResult{}, errors.New("no result")
}
