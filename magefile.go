// +build mage

package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/mholt/archiver"
)

var (
	executableName = "wiki"
	outputPath     = "dist"
	architectures  = []Architecture{
		{OS: "linux", Arch: "amd64", ArchiveType: ".tar.gz"},
		{OS: "linux", Arch: "arm64", ArchiveType: ".tar.gz"},
		{OS: "darwin", Arch: "amd64", ArchiveType: ".tar.gz"},
		{OS: "darwin", Arch: "arm64", ArchiveType: ".tar.gz"},
		{OS: "windows", Arch: "amd64", BinarySuffix: ".exe", ArchiveType: ".zip"},
	}
)

var goexe = "go"

func Archive() error {
	mg.Deps(Build, Notices)
	fmt.Printf("Creating archives\n")
	version, err := getTag()
	if err != nil {
		return err
	}
	for _, architecture := range architectures {
		binaryName := fmt.Sprintf("%s_%s_%s_%s", executableName, architecture.OS, architecture.Arch, version)
		outputName := filepath.Join(outputPath, binaryName)
		err := archiver.Archive([]string{
			"dist/"+binaryName+architecture.BinarySuffix,
			"dist/notices",
		}, outputName+architecture.ArchiveType)
		if err != nil {
			log.Printf("Error archiving: %s%s: %s", architecture.OS, architecture.Arch, err.Error())
		}
	}
	return nil
}

func Notices() error {
	fmt.Printf("Getting licenses\n")
	buildtime, err := getBuildtime("HEAD")
	if err != nil {
		return err
	}
	noticesPath := filepath.Join(outputPath, "notices")
	err = sh.Run(goexe, "get", "")
	if err != nil {
		return err
	}
	err = sh.Run(goexe, "run", "github.com/google/go-licenses", "save", "./...", fmt.Sprintf("--save_path=%s", noticesPath), "--force")
	if err != nil {
		return err
	}
	return filepath.WalkDir(noticesPath, setTimeFunc(*buildtime))
}

func Build() error {
	fmt.Printf("Compiling binaries\n")
	err := os.Setenv("GO111MODULE", "on")
	if err != nil {
		return err
	}
	err = os.Setenv("CGOENABLED", "0")
	if err != nil {
		return err
	}
	buildTime, err := getBuildtime("HEAD")
	if err != nil {
		return err
	}
	version, err := getTag()
	if err != nil {
		return err
	}
	options, err := getCompileOptions(version)
	if err != nil {
		return err
	}
	if err := sh.Run("go", "mod", "download"); err != nil {
		return err
	}
	for index := range architectures {
		compile := Compile{
			arch:         architectures[index],
			options:      options,
			version:      version,
			binaryName:   executableName,
			binaryFolder: outputPath,
			buildTime:    *buildTime,
		}
		err = build(compile)
		if err != nil {
			log.Printf("Error building: %s%s: %s", architectures[index].OS, architectures[index].Arch, err.Error())
		}
	}
	return nil
}

func setTimeFunc(buildtime time.Time) func(path string, info fs.DirEntry, err error) error {
	return func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		err = os.Chtimes(path, buildtime, buildtime)
		if err != nil {
			return err
		}
		return nil
	}
}

func getBuildtime(commit string) (*time.Time, error) {
	var err error
	buildTime := time.Now()
	envBuildtime := os.Getenv("BUILDTIME")
	if envBuildtime == "" {
		envBuildtime, err = getCommitTimestamp(commit)
		if err != nil {
			return nil, err
		}
	}
	buildTime, err = time.Parse("2006-01-02 15:04:05 -0700", envBuildtime)
	if err != nil {
		return nil, err
	}
	return &buildTime, nil
}

func getCommitTimestamp(commit string) (string, error) {
	s, err := sh.Output("git", "show", "-s", "--format=%ci", commit)
	if err != nil {
		return "", err
	}
	return s, nil
}

func getTag() (string, error) {
	_, err := sh.Output("git", "fetch", "--tags")
	if err != nil {
		return "", err
	}
	s, err := sh.Output("git", "describe", "--tags")
	if err != nil {
		return "", err
	}
	return s, nil
}

func getCompileOptions(version string) (options CompilerOptions, err error) {
	options = CompilerOptions{
		GCFlags: []string{
			`./dontoptimizeme=-N`,
		},
		LDFlags: []string{
			`-s`,
			`-w`,
			fmt.Sprintf(`-X "main.version=%s"`, version),
		},
		MiscFlags: []string{
			`-trimpath`,
		},
	}
	return
}

func build(compile Compile) error {
	err := os.Setenv("GOOS", compile.arch.OS)
	if err != nil {
		return err
	}
	err = os.Setenv("GOARCH", compile.arch.Arch)
	if err != nil {
		return err
	}
	err = sh.RunV(goexe, compile.options.getAllFlags(compile.getOutputName())...)
	if err != nil {
		return err
	}
	err = filepath.WalkDir(compile.getOutputName(), setTimeFunc(compile.buildTime))
	if err != nil {
		return err
	}
	return nil
}

type Architecture struct {
	OS           string
	Arch         string
	BinarySuffix string
	ArchiveType  string
}

type CompilerOptions struct {
	GCFlags   []string
	LDFlags   []string
	MiscFlags []string
}

func (c *CompilerOptions) getAllFlags(output string) []string {
	buildFlags := []string{
		"build",
	}
	buildFlags = append(buildFlags, c.MiscFlags...)
	buildFlags = append(buildFlags, "-gcflags="+strings.Join(c.GCFlags, " "))
	buildFlags = append(buildFlags, "-ldflags="+strings.Join(c.LDFlags, " "))
	buildFlags = append(buildFlags, "-o")
	buildFlags = append(buildFlags, output)
	buildFlags = append(buildFlags, ".")
	return buildFlags
}

type Compile struct {
	arch         Architecture
	options      CompilerOptions
	version      string
	binaryName   string
	binaryFolder string
	buildTime    time.Time
}

func (c *Compile) getOutputName() string {
	binaryName := fmt.Sprintf("%s_%s_%s_%s%s", c.binaryName, c.arch.OS, c.arch.Arch, c.version, c.arch.BinarySuffix)
	return filepath.Join(c.binaryFolder, binaryName)
}