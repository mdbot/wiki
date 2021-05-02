// +build mage

package main

import (
	"fmt"
	"io/fs"
	"io/ioutil"
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
	timeFormat = "2006-01-02 15:04:05 -0700"
)

var goexe = "go"
var dockerexe = "docker"
var docker = false

var buildTag = "unknown"
var buildTime = time.Time{}

func SetBuildVersion() error {
	var err error
	buildTag, err = getTag()
	if err != nil {
		return err
	}
	return nil
}

func SetBuildTime() error {
	var err error
	commitTimestamp, err := getCommitTimestamp("HEAD")
	if err != nil {
		return err
	}
	buildTime, err = time.Parse(timeFormat, commitTimestamp)
	if err != nil {
		return err
	}
	return nil
}

func Release() error {
	mg.Deps(Docker, Archive)
	return nil
}

func Docker() error {
	docker = true
	bytesRead, err := ioutil.ReadFile("gorelease.Dockerfile")
	if err != nil {
		log.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(outputPath, "Dockerfile"), bytesRead, 0755)
	if err != nil {
		log.Fatal(err)
	}
	mg.Deps(Notices, SetBuildTime, SetBuildVersion, LinuxAmd64)
	return nil
}

func BuildDocker() error {
	if !docker {
		return nil
	}
	fmt.Printf("Building docker container\n")
	err := sh.Run(dockerexe, "build", "-t", "test2", outputPath)
	if err != nil {
		return err
	}
	return nil
}

func Archive() error {
	mg.Deps(Notices, Binaries)
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
	err = sh.Run(goexe, "get", "github.com/google/go-licenses")
	if err != nil {
		return err
	}
	err = sh.Run("go-licenses", "save", "./...", fmt.Sprintf("--save_path=%s", noticesPath), "--force")
	if err != nil {
		return err
	}
	return filepath.WalkDir(noticesPath, setTimeFunc(*buildtime))
}

func Binaries() error {
	mg.Deps(LinuxAmd64,	LinuxArm64,	DarwinAmd64, DarwinArm64, WindowsAmd64)
	return nil
}

func WindowsAmd64() error {
	fmt.Printf("Building Windows AMD64\n")
	mg.Deps(SetBuildVersion, SetBuildTime)
	options, err := getCompileOptions()
	if err != nil {
		return err
	}
	return build(Compile{
		arch:         Architecture{
			OS:           "windows",
			Arch:         "amd64",
			BinarySuffix: ".exe",
			ArchiveType:  ".zip",
		},
		options:      options,
		version:      buildTag,
		binaryName:   executableName,
		binaryFolder: outputPath,
		buildTime:    buildTime,
	})
}

func LinuxAmd64() error {
	fmt.Printf("Building Linux AMD64\n")
	mg.Deps(SetBuildVersion, SetBuildTime)
	options, err := getCompileOptions()
	if err != nil {
		return err
	}
	err = build(Compile{
		arch:         Architecture{
			OS:           "linux",
			Arch:         "amd64",
			BinarySuffix: "",
			ArchiveType:  ".tar.gz",
		},
		options:      options,
		version:      buildTag,
		binaryName:   executableName,
		binaryFolder: outputPath,
		buildTime:    buildTime,
	})
	if err != nil {
		return err
	}
	mg.Deps(BuildDocker)
	return nil
}

func LinuxArm64() error {
	fmt.Printf("Building Linux ARM64\n")
	mg.Deps(SetBuildVersion, SetBuildTime)
	options, err := getCompileOptions()
	if err != nil {
		return err
	}
	return build(Compile{
		arch:         Architecture{
			OS:           "linux",
			Arch:         "arm64",
			BinarySuffix: "",
			ArchiveType:  ".tar.gz",
		},
		options:      options,
		version:      buildTag,
		binaryName:   executableName,
		binaryFolder: outputPath,
		buildTime:    buildTime,
	})
}

func DarwinAmd64() error {
	fmt.Printf("Building Darwin AMD64\n")
	mg.Deps(SetBuildVersion, SetBuildTime)
	options, err := getCompileOptions()
	if err != nil {
		return err
	}
	return build(Compile{
		arch:         Architecture{
			OS:           "darwin",
			Arch:         "amd64",
			BinarySuffix: "",
			ArchiveType:  ".tar.gz",
		},
		options:      options,
		version:      buildTag,
		binaryName:   executableName,
		binaryFolder: outputPath,
		buildTime:    buildTime,
	})
}

func DarwinArm64() error {
	fmt.Printf("Building Darwin ARM64\n")
	mg.Deps(SetBuildVersion, SetBuildTime)
	options, err := getCompileOptions()
	if err != nil {
		return err
	}
	return build(Compile{
		arch:         Architecture{
			OS:           "darwin",
			Arch:         "arm64",
			BinarySuffix: "",
			ArchiveType:  ".tar.gz",
		},
		options:      options,
		version:      buildTag,
		binaryName:   executableName,
		binaryFolder: outputPath,
		buildTime:    buildTime,
	})
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

func getCompileOptions() (options CompilerOptions, err error) {
	options = CompilerOptions{
		GCFlags: []string{
			`./dontoptimizeme=-N`,
		},
		LDFlags: []string{
			`-s`,
			`-w`,
			fmt.Sprintf(`-X "main.version=%s"`, buildTag),
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