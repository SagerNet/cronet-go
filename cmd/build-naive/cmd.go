package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// Target represents a build target platform
type Target struct {
	OS   string // gn target_os: linux, mac, win, android, ios
	CPU  string // gn target_cpu: x64, arm64, x86, arm
	GOOS string // Go GOOS
	ARCH string // Go GOARCH
}

var allTargets = []Target{
	{OS: "linux", CPU: "x64", GOOS: "linux", ARCH: "amd64"},
	{OS: "linux", CPU: "arm64", GOOS: "linux", ARCH: "arm64"},
	{OS: "mac", CPU: "x64", GOOS: "darwin", ARCH: "amd64"},
	{OS: "mac", CPU: "arm64", GOOS: "darwin", ARCH: "arm64"},
	{OS: "win", CPU: "x64", GOOS: "windows", ARCH: "amd64"},
	{OS: "win", CPU: "arm64", GOOS: "windows", ARCH: "arm64"},
	{OS: "ios", CPU: "arm64", GOOS: "ios", ARCH: "arm64"},
	{OS: "android", CPU: "arm64", GOOS: "android", ARCH: "arm64"},
	{OS: "android", CPU: "x64", GOOS: "android", ARCH: "amd64"},
	{OS: "android", CPU: "arm", GOOS: "android", ARCH: "arm"},
	{OS: "android", CPU: "x86", GOOS: "android", ARCH: "386"},
}

var (
	projectRoot string
	naiveRoot   string
	srcRoot     string
	targetStr   string
)

var mainCommand = &cobra.Command{
	Use:              "build-naive",
	Short:            "Build tool for cronet-go naiveproxy integration",
	PersistentPreRun: preRun,
}

func init() {
	log.SetFlags(0)
	log.SetPrefix("[build] ")
	mainCommand.PersistentFlags().StringVarP(&targetStr, "targets", "t", "", "Comma-separated list of targets (e.g., linux/amd64,darwin/arm64). Empty means host only.")
}

func preRun(cmd *cobra.Command, args []string) {
	// Find project root (directory containing go.mod)
	workingDirectory, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get working directory: %v", err)
	}

	for directory := workingDirectory; ; directory = filepath.Dir(directory) {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			projectRoot = directory
			break
		}
		if directory == filepath.Dir(directory) {
			log.Fatal("could not find project root (go.mod)")
		}
	}

	naiveRoot = filepath.Join(projectRoot, "naiveproxy")
	srcRoot = filepath.Join(naiveRoot, "src")
}

func parseTargets() []Target {
	if targetStr == "" {
		// Default to host platform
		hostOS := runtime.GOOS
		hostArch := runtime.GOARCH
		for _, t := range allTargets {
			if t.GOOS == hostOS && t.ARCH == hostArch {
				return []Target{t}
			}
		}
		log.Fatalf("unsupported host platform: %s/%s", hostOS, hostArch)
	}

	if targetStr == "all" {
		return allTargets
	}

	var targets []Target
	for _, part := range strings.Split(targetStr, ",") {
		part = strings.TrimSpace(part)
		parts := strings.Split(part, "/")
		if len(parts) != 2 {
			log.Fatalf("invalid target format: %s (expected os/arch)", part)
		}
		goos, goarch := parts[0], parts[1]
		found := false
		for _, t := range allTargets {
			if t.GOOS == goos && t.ARCH == goarch {
				targets = append(targets, t)
				found = true
				break
			}
		}
		if !found {
			log.Fatalf("unsupported target: %s/%s", goos, goarch)
		}
	}
	return targets
}

// hostToCPU converts Go GOARCH to GN cpu
func hostToCPU(goarch string) string {
	switch goarch {
	case "amd64":
		return "x64"
	case "arm64":
		return "arm64"
	case "386":
		return "x86"
	case "arm":
		return "arm"
	default:
		return goarch
	}
}

func runCommand(directory string, name string, args ...string) {
	command := exec.Command(name, args...)
	command.Dir = directory
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err := command.Run()
	if err != nil {
		log.Fatalf("command failed: %s %s: %v", name, strings.Join(args, " "), err)
	}
}

func runCommandOutput(directory string, name string, args ...string) string {
	command := exec.Command(name, args...)
	command.Dir = directory
	var stdout bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = os.Stderr
	err := command.Run()
	if err != nil {
		log.Fatalf("command failed: %s %s: %v", name, strings.Join(args, " "), err)
	}
	return stdout.String()
}

func copyFile(source, destination string) {
	sourceFile, err := os.Open(source)
	if err != nil {
		log.Fatalf("failed to open %s: %v", source, err)
	}
	defer sourceFile.Close()

	err = os.MkdirAll(filepath.Dir(destination), 0o755)
	if err != nil {
		log.Fatalf("failed to create directory for %s: %v", destination, err)
	}

	destinationFile, err := os.Create(destination)
	if err != nil {
		log.Fatalf("failed to create %s: %v", destination, err)
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		log.Fatalf("failed to copy %s to %s: %v", source, destination, err)
	}
}

func copyDirectory(source, destination string) {
	os.MkdirAll(destination, 0o755)
	entries, err := os.ReadDir(source)
	if err != nil {
		return
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(source, entry.Name())
		destinationPath := filepath.Join(destination, entry.Name())
		if entry.IsDir() {
			copyDirectory(sourcePath, destinationPath)
		} else {
			copyFile(sourcePath, destinationPath)
		}
	}
}

func copyGlob(sourceDirectory, destinationDirectory, pattern string) {
	matches, _ := filepath.Glob(filepath.Join(sourceDirectory, pattern))
	for _, source := range matches {
		relative, _ := filepath.Rel(sourceDirectory, source)
		destination := filepath.Join(destinationDirectory, relative)

		info, err := os.Stat(source)
		if err != nil {
			continue
		}
		if info.IsDir() {
			copyDirectory(source, destination)
		} else {
			copyFile(source, destination)
		}
	}
}
