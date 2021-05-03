// +build mage

package main

import (
	"crypto/sha256"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/mholt/archiver"
)

const (
	ProjectName = "wiki"
	ProjectGroup = "mdbot"
	DistFolder  = "dist"
	TimeFormat  = "2006-01-02 15:04:05 -0700"
	GoExe       = "go"
	DockerEXE   = "docker"
	GitExe      = "git"
)

var (
	arches = []Architecture{
		{OS: "linux", Arch: "amd64", ArchiveType: ".tar.gz"},
		{OS: "linux", Arch: "arm64", ArchiveType: ".tar.gz"},
		{OS: "darwin", Arch: "amd64", ArchiveType: ".tar.gz"},
		{OS: "darwin", Arch: "arm64", ArchiveType: ".tar.gz"},
		{OS: "windows", Arch: "amd64", BinarySuffix: ".exe", ArchiveType: ".zip"},
	}
	registries = []string{
		"index.docker.io",
		"ghcr.io",
	}
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
	isTag      = false
	semverTags []string
	dockerTags []string
	buildTag   = "unknown"
	buildTime  = time.Time{}

	Default = Release.Build
)

type Build mg.Namespace
type Release mg.Namespace

func init() {
	err := setBuildTime()
	if err != nil {
		fmt.Printf("Error getting build time: %s", err.Error())
		os.Exit(1)
	}
	err = setBuildVersion()
	if err != nil {
		fmt.Printf("Error getting build version: %s", err.Error())
		os.Exit(1)
	}
	err = setSemVerTags()
	if err != nil {
		fmt.Printf("Error getting semantic versions: %s", err.Error())
		os.Exit(1)
	}
	setDockerTags()
}

func (Release) BuildAndPush() error {
	mg.Deps(Release.Build)
	for _, dockerTag := range dockerTags {
		err := sh.Run(DockerEXE, "tag", ProjectName, dockerTag)
		if err != nil {
			return err
		}
		err = sh.Run(DockerEXE, "push", dockerTag)
		if err != nil {
			return err
		}
	}
	return nil
}

func (Release) Build() error {
	mg.Deps(Release.Docker, Release.Archive)
	return nil
}

func (Release) Docker() error {
	mg.Deps(Release.Notices, Build.LinuxAmd64)
	fmt.Printf("Building docker container\n")
	dockerTemplate := template.Must(template.ParseFiles("dockerfile.gotpl"))
	binaryName := fmt.Sprintf("%s_%s_%s_%s", ProjectName, "linux", "amd64", buildTag)
	binaryPath := filepath.Join("binaries", binaryName)
	file, err := os.OpenFile(filepath.Join(DistFolder, "Dockerfile"), os.O_TRUNC|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	err = dockerTemplate.Execute(file, binaryPath)
	if err != nil {
		return err
	}
	err = sh.Run(DockerEXE, "build", "-t", ProjectName, DistFolder)
	if err != nil {
		return err
	}
	err = sh.Rm(filepath.Join(DistFolder, "Dockerfile"))
	if err != nil {
		return err
	}
	return nil
}

func (Release) Archive() error {
	mg.Deps(Release.Notices, Build.All)
	fmt.Printf("Creating archives\n")
	for _, architecture := range arches {
		binaryName := fmt.Sprintf("%s_%s_%s_%s", ProjectName, architecture.OS, architecture.Arch, buildTag)
		outputName := filepath.Join(DistFolder, "archives", binaryName)
		err := archiver.Archive([]string{
			"dist/" + binaryName + architecture.BinarySuffix,
			"dist/notices",
		}, outputName+architecture.ArchiveType)
		if err != nil {
			log.Printf("Error archiving: %s%s: %s", architecture.OS, architecture.Arch, err.Error())
		}
		var data []byte
		data, err = os.ReadFile(outputName + architecture.ArchiveType)
		if err != nil {
			log.Printf("Error reading archive: %s%s: %s", architecture.OS, architecture.Arch, err.Error())
		}
		checksum := sha256.Sum256(data)
		err = os.WriteFile(outputName+"_checksum.sha256", []byte(fmt.Sprintf("%x", checksum)), 0644)
		if err != nil {
			log.Printf("Error writing checksum: %s%s: %s", architecture.OS, architecture.Arch, err.Error())
		}
	}
	return nil
}

func (Release) Notices() error {
	fmt.Printf("Getting licenses\n")
	noticesPath := filepath.Join(DistFolder, "notices")
	err := sh.Run(GoExe, "get", "")
	if err != nil {
		return err
	}
	err = sh.Run(GoExe, "get", "github.com/google/go-licenses")
	if err != nil {
		return err
	}
	err = sh.Run("go-licenses", "save", "./...", fmt.Sprintf("--save_path=%s", noticesPath), "--force")
	if err != nil {
		return err
	}
	return filepath.WalkDir(noticesPath, setTimeFunc(buildTime))
}

func (Build) Clean() error {
	err := sh.Rm(filepath.Join(DistFolder, "binaries"))
	if err != nil {
		return err
	}
	err = sh.Rm(filepath.Join(DistFolder, "archives"))
	if err != nil {
		return err
	}
	err = sh.Rm(filepath.Join(DistFolder, "releases"))
	if err != nil {
		return err
	}
	err = sh.Rm(filepath.Join(DistFolder, "notices"))
	if err != nil {
		return err
	}
	return nil
}

func (Build) All() error {
	mg.Deps(Build.LinuxAmd64, Build.LinuxArm64, Build.DarwinAmd64, Build.DarwinArm64, Build.WindowsAmd64)
	return nil
}

func (Build) WindowsAmd64() error {
	fmt.Printf("Building Windows AMD64\n")
	return build(Architecture{
		OS:           "windows",
		Arch:         "amd64",
		BinarySuffix: ".exe",
		ArchiveType:  ".zip",
	})
}

func (Build) LinuxAmd64() error {
	fmt.Printf("Building Linux AMD64\n")
	return build(Architecture{
		OS:           "linux",
		Arch:         "amd64",
		BinarySuffix: "",
		ArchiveType:  ".tar.gz",
	})
}

func (Build) LinuxArm64() error {
	fmt.Printf("Building Linux ARM64\n")
	return build(Architecture{
		OS:           "linux",
		Arch:         "arm64",
		BinarySuffix: "",
		ArchiveType:  ".tar.gz",
	})
}

func (Build) DarwinAmd64() error {
	fmt.Printf("Building Darwin AMD64\n")
	return build(Architecture{
		OS:           "darwin",
		Arch:         "amd64",
		BinarySuffix: "",
		ArchiveType:  ".tar.gz",
	})
}

func (Build) DarwinArm64() error {
	fmt.Printf("Building Darwin ARM64\n")
	return build(Architecture{
		OS:           "darwin",
		Arch:         "arm64",
		BinarySuffix: "",
		ArchiveType:  ".tar.gz",
	})
}

func setBuildVersion() error {
	var err error
	buildTag, err = getTag()
	if err != nil {
		return err
	}
	var exactTag string
	exactTag, isTag, err = getExactTag()
	if isTag {
		fmt.Printf("Tagged build: %s\n", exactTag)
	} else {
		fmt.Printf("Snapshot build: %s\n", buildTag)
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

func setBuildTime() error {
	var err error
	cmd := exec.Command(GitExe, "show", "-s", "--format=%ci", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	commitTimestamp := strings.TrimSpace(string(output))
	buildTime, err = time.Parse(TimeFormat, commitTimestamp)
	if err != nil {
		return err
	}
	return nil
}

func setSemVerTags() error {
	if !isTag {
		semverTags = append(semverTags, "latest")
		return nil
	}
	buildTag = strings.TrimPrefix(buildTag, "v")
	semVer, err := semver.NewVersion(buildTag)
	if err != nil {
		fmt.Printf("Not a semver release: %s\n", err)
		return err
	}
	semverTags = append(semverTags, fmt.Sprintf("%d.%d.%d", semVer.Major, semVer.Minor, semVer.Patch))
	semverTags = append(semverTags, fmt.Sprintf("%d.%d", semVer.Major, semVer.Minor))
	semverTags = append(semverTags, fmt.Sprintf("%d", semVer.Major))
	return nil
}

func setDockerTags() {
	for _, semverTag := range semverTags {
		for _, registry := range registries {
			dockerTags = append(dockerTags, fmt.Sprintf("%s/%s/%s:%s", registry, ProjectGroup, ProjectName, semverTag))
		}
	}
}

func getTag() (string, error) {
	cmd := exec.Command(GitExe, "fetch", "--tags")
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	cmd = exec.Command(GitExe, "describe", "--tags")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func getExactTag() (string, bool, error) {
	cmd := exec.Command(GitExe, "fetch", "--tags")
	err := cmd.Run()
	if err != nil {
		return "", false, err
	}
	cmd = exec.Command(GitExe, "describe", "--exact-match", "--tags")
	output, err := cmd.Output()
	if err != nil {
		return "", false, err
	}
	return strings.TrimSpace(string(output)), true, err
}

func build(arch Architecture) error {
	err := os.Setenv("GOOS", arch.OS)
	if err != nil {
		return err
	}
	err = os.Setenv("GOARCH", arch.Arch)
	if err != nil {
		return err
	}
	outputName := arch.getOutputName()
	err = sh.RunV(GoExe, options.getAllFlags(outputName)...)
	if err != nil {
		return err
	}
	err = filepath.WalkDir(outputName, setTimeFunc(buildTime))
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

func (a *Architecture) getOutputName() string {
	return filepath.Join(DistFolder, "binaries", fmt.Sprintf("%s_%s_%s_%s%s", ProjectName, a.OS, a.Arch, buildTag, a.BinarySuffix))
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
