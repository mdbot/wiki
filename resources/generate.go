package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/evanw/esbuild/pkg/api"
)

func main() {
	dir, err := os.MkdirTemp("", "wiki-build-*")
	if err != nil {
		panic(err)
	}

	resp, err := http.Get("https://codemirror.net/codemirror-5.60.0.zip")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// Extract zip to dir
	// Copy codemirror.js to dir
	// Run builder with the dir as the base oath or whatever
	_ = dir

	res := api.Build(api.BuildOptions{
		Bundle:            true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		MinifyWhitespace:  true,
		Write:             true,
		EntryPoints:       []string{"resources/codemirror.js"},
		Outfile:           "resources/static/codemirror-min.js",
	})

	fmt.Printf("%v\n", res)
}
