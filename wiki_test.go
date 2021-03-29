package main

import (
	"path/filepath"
	"testing"
)

func Test_resolvePath(t *testing.T) {
	type args struct {
		base  string
		title string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"basic file", args{"repo", "title"}, filepath.Join("repo", "title.md"), false},
		{"sub directory", args{"repo", "some/folder/title"}, filepath.Join("repo", "some", "folder", "title.md"), false},
		{"collapse dirs", args{"repo", "some/folder/../../title"}, filepath.Join("repo", "title.md"), false},
		{"normalise dots", args{"repo", "./title"}, filepath.Join("repo", "title.md"), false},
		{"directory escape", args{"repo", "../title"}, "", true},
		{"directory escape", args{"repo", "foo/../../title"}, "", true},
		{"relative git directory", args{"repo", "./.git/refs"}, "", true},
		{"git directory", args{"repo", ".git/refs"}, "", true},
		{"git subdirectory", args{"repo", "foo/bar/.git/refs"}, "", true},
		{"mixed-case git", args{"repo", "foo/bar/.GiT/refs"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolvePath(tt.args.base, tt.args.title)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolvePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("resolvePath() got = %v, want %v", got, tt.want)
			}
		})
	}
}
