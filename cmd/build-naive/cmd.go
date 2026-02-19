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
	OS          string // gn target_os: linux, mac, win, android, ios, openwrt
	CPU         string // gn target_cpu: x64, arm64, x86, arm
	GOOS        string // Go GOOS
	ARCH        string // Go GOARCH
	Libc        string // C library: "" (default), "glibc", or "musl" (Linux only)
	Platform    string // Apple: "iphoneos", "tvos" (GN target_platform)
	Environment string // Apple: "device", "simulator" (GN target_environment)
}

var allTargets = []Target{
	{OS: "linux", CPU: "x64", GOOS: "linux", ARCH: "amd64"},
	{OS: "linux", CPU: "arm64", GOOS: "linux", ARCH: "arm64"},
	{OS: "linux", CPU: "x86", GOOS: "linux", ARCH: "386"},
	{OS: "linux", CPU: "arm", GOOS: "linux", ARCH: "arm"},
	{OS: "linux", CPU: "loong64", GOOS: "linux", ARCH: "loong64"},
	{OS: "mac", CPU: "x64", GOOS: "darwin", ARCH: "amd64"},
	{OS: "mac", CPU: "arm64", GOOS: "darwin", ARCH: "arm64"},
	{OS: "win", CPU: "x64", GOOS: "windows", ARCH: "amd64"},
	{OS: "win", CPU: "arm64", GOOS: "windows", ARCH: "arm64"},
	{OS: "ios", CPU: "arm64", GOOS: "ios", ARCH: "arm64", Platform: "iphoneos", Environment: "device"},
	{OS: "ios", CPU: "arm64", GOOS: "ios", ARCH: "arm64", Platform: "iphoneos", Environment: "simulator"},
	{OS: "ios", CPU: "x64", GOOS: "ios", ARCH: "amd64", Platform: "iphoneos", Environment: "simulator"},
	{OS: "ios", CPU: "arm64", GOOS: "ios", ARCH: "arm64", Platform: "tvos", Environment: "device"},
	{OS: "ios", CPU: "arm64", GOOS: "ios", ARCH: "arm64", Platform: "tvos", Environment: "simulator"},
	{OS: "ios", CPU: "x64", GOOS: "ios", ARCH: "amd64", Platform: "tvos", Environment: "simulator"},
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
	libcStr     string
)

var mainCommand = &cobra.Command{
	Use:              "build-naive",
	Short:            "Build tool for cronet-go naiveproxy integration",
	PersistentPreRun: preRun,
}

func init() {
	log.SetFlags(0)
	log.SetPrefix("[build] ")
	mainCommand.PersistentFlags().StringVarP(&targetStr, "target", "t", "", "Comma-separated list of targets (e.g., linux/amd64,darwin/arm64). Empty means host only.")
	mainCommand.PersistentFlags().StringVar(&libcStr, "libc", "", "C library for Linux: glibc (default) or musl")
}

func preRun(cmd *cobra.Command, args []string) {
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
	if libcStr != "" && libcStr != "glibc" && libcStr != "musl" {
		log.Fatalf("invalid libc: %s (expected glibc or musl)", libcStr)
	}

	if targetStr == "" {
		hostOS := runtime.GOOS
		hostArch := runtime.GOARCH
		for _, t := range allTargets {
			if t.GOOS == hostOS && t.ARCH == hostArch {
				target := t
				target = applyLibc(target)
				return []Target{target}
			}
		}
		log.Fatalf("unsupported host platform: %s/%s", hostOS, hostArch)
	}

	if targetStr == "all" {
		targets := make([]Target, len(allTargets))
		for i, t := range allTargets {
			targets[i] = applyLibc(t)
		}
		return targets
	}

	var targets []Target
	for _, part := range strings.Split(targetStr, ",") {
		part = strings.TrimSpace(part)
		parts := strings.Split(part, "/")
		if len(parts) < 2 || len(parts) > 3 {
			log.Fatalf("invalid target format: %s (expected os/arch or os/arch/variant)", part)
		}
		goos, goarch := parts[0], parts[1]

		// Handle iOS and tvOS targets with optional simulator suffix
		if goos == "ios" || goos == "tvos" {
			target := parseAppleTarget(goos, goarch, parts)
			targets = append(targets, target)
			continue
		}

		if len(parts) != 2 {
			log.Fatalf("invalid target format: %s (expected os/arch)", part)
		}

		found := false
		for _, t := range allTargets {
			if t.GOOS == goos && t.ARCH == goarch && t.Platform == "" {
				target := applyLibc(t)
				targets = append(targets, target)
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

// parseAppleTarget parses iOS/tvOS target strings
// Formats:
//   - ios/arm64 -> iOS device arm64
//   - ios/arm64/simulator -> iOS simulator arm64
//   - ios/amd64 -> iOS simulator amd64 (only simulator supports amd64)
//   - tvos/arm64 -> tvOS device arm64
//   - tvos/arm64/simulator -> tvOS simulator arm64
//   - tvos/amd64 -> tvOS simulator amd64
func parseAppleTarget(goos, goarch string, parts []string) Target {
	isTvOS := goos == "tvos"
	platform := "iphoneos"
	if isTvOS {
		platform = "tvos"
	}

	environment := "device"
	if len(parts) == 3 && parts[2] == "simulator" {
		environment = "simulator"
	} else if goarch == "amd64" {
		// amd64 is only available for simulator
		environment = "simulator"
	}

	for _, t := range allTargets {
		if t.GOOS == "ios" && t.ARCH == goarch && t.Platform == platform && t.Environment == environment {
			return t
		}
	}

	log.Fatalf("unsupported target: %s/%s", goos, goarch)
	return Target{}
}

func applyLibc(target Target) Target {
	if libcStr == "" || libcStr == "glibc" {
		return target
	}

	if target.GOOS != "linux" {
		log.Fatalf("--libc=musl is only supported for Linux targets, not %s", target.GOOS)
	}

	// For musl, we use openwrt as the target OS
	target.OS = "openwrt"
	target.Libc = "musl"
	return target
}

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
	case "loong64":
		return "loong64"
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
