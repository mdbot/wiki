package main

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

func main() {
	dir, err := os.MkdirTemp("", "wiki-build-*")
	if err != nil {
		panic(err)
	}

	codemirrorBytes := downloadFile("https://codemirror.net/codemirror-5.60.0.zip")
	extractZip(codemirrorBytes, dir, "codemirror-5.60.0", "codemirror")
	copyFile("resources/editor.js", dir)
	copyFile("resources/editor.css", dir)

	outFile, _ := filepath.Abs("./resources/static/")

	res := api.Build(api.BuildOptions{
		Bundle:            true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		MinifyWhitespace:  true,
		Write:             true,
		AbsWorkingDir:     dir,
		EntryPoints:       []string{"editor.js", "editor.css"},
		Outdir:            outFile,
	})

	if len(res.Errors) > 0 {
		panic(res.Errors[0].Text)
	}

	os.RemoveAll(dir)
}

func downloadFile(url string) []byte {
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	return b
}

func copyFile(file, dir string) {
	bs, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.Base(file)), bs, os.FileMode(0644)); err != nil {
		panic(err)
	}
}

func extractZip(b []byte, dir string, replacements ...string) {
	replacer := strings.NewReplacer(replacements...)

	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		panic(err)
	}

	for i := range zr.File {
		func(f *zip.File) {
			if f.FileInfo().IsDir() {
				return
			}

			file, err := f.Open()
			if err != nil {
				panic(err)
			}
			defer file.Close()

			bs, err := io.ReadAll(file)
			if err != nil {
				panic(err)
			}

			target := replacer.Replace(filepath.Join(dir, f.Name))

			if err := os.MkdirAll(filepath.Dir(target), os.FileMode(0755)); err != nil {
				panic(err)
			}

			if err := os.WriteFile(target, bs, os.FileMode(0644)); err != nil {
				panic(err)
			}
		}(zr.File[i])
	}
}
