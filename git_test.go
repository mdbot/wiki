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
		name     string
		args     args
		wantFile string
		wantGit  string
		wantErr  bool
	}{
		{
			"basic file",
			args{"repo", "title.md"},
			filepath.Join("repo", "title.md"),
			"title.md",
			false,
		},
		{
			"sub directory",
			args{"repo", "some/folder/title.md"},
			filepath.Join("repo", "some", "folder", "title.md"),
			filepath.Join("some", "folder", "title.md"),
			false,
		},
		{
			"collapse dirs",
			args{"repo", "some/folder/../../title.md"},
			filepath.Join("repo", "title.md"),
			filepath.Join("title.md"),
			false,
		},
		{
			"normalise dots",
			args{"repo", "./title.md"},
			filepath.Join("repo", "title.md"),
			filepath.Join("title.md"),
			false,
		},
		{
			"directory escape",
			args{"repo", "../title.md"},
			"",
			"",
			true,
		},
		{
			"nested directory escape",
			args{"repo", "foo/../../title.md"},
			"",
			"",
			true,
		},
		{
			"relative git directory",
			args{"repo", "./.git/title.md"},
			"",
			"",
			true,
		},
		{
			"git subdirectory",
			args{"repo", "foo/bar/.git/title.md"},
			"",
			"",
			true,
		},
		{
			"git mixed-case",
			args{"repo", "foo/bar/.gIt/title.md"},
			"",
			"",
			true,
		},
		{
			"wiki subdirectory",
			args{"repo", "foo/bar/.wiki/title.md"},
			"",
			"",
			true,
		},
		{
			"wiki mixed-case",
			args{"repo", "foo/bar/.WIKi/title.md"},
			"",
			"",
			true,
		},
		{
			"mixed case directory",
			args{"repo", "Foo/bar.md"},
			filepath.Join("repo", "foo", "bar.md"),
			filepath.Join("foo", "bar.md"),
			false,
		},
		{
			"mixed case file",
			args{"repo", "foo/Bar.md"},
			filepath.Join("repo", "foo", "bar.md"),
			filepath.Join("foo", "bar.md"),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFilePath, gotGitPath, err := resolvePath(tt.args.base, tt.args.title)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolvePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotFilePath != tt.wantFile {
				t.Errorf("resolvePath() got file path = %v, want %v", gotFilePath, tt.wantFile)
				return
			}
			if gotGitPath != tt.wantGit {
				t.Errorf("resolvePath() got git path = %v, want %v", gotGitPath, tt.wantGit)
				return
			}
		})
	}
}
