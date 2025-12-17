package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	commandExtractLib = &cobra.Command{
		Use:   "extract-lib",
		Short: "Extract dynamic libraries from Go module dependencies",
		Long: `Extract dynamic libraries (.so/.dll) from Go module dependencies.

This command downloads the cronet-go lib submodule for the specified target
and extracts the dynamic library to the output directory.

Supported targets:
  - linux/amd64, linux/arm64, linux/386, linux/arm (glibc only)
  - windows/amd64, windows/arm64

Not supported (use static linking via CGO instead):
  - Linux musl (static only)
  - macOS, iOS, tvOS, Android`,
		Run: func(cmd *cobra.Command, args []string) {
			targets := parseTargets()
			extractLibraries(targets)
		},
	}
	outputDirectory string
	outputName      string
)

func init() {
	mainCommand.AddCommand(commandExtractLib)
	commandExtractLib.Flags().StringVarP(&outputDirectory, "output", "o", ".",
		"Output directory for extracted libraries")
	commandExtractLib.Flags().StringVarP(&outputName, "name", "n", "",
		"Output filename (default: libcronet.so or libcronet.dll)")
}

func extractLibraries(targets []Target) {
	if err := os.MkdirAll(outputDirectory, 0o755); err != nil {
		log.Fatalf("failed to create output directory: %v", err)
	}

	for _, target := range targets {
		extractLibrary(target)
	}

	log.Print("Extract complete!")
}

func extractLibrary(target Target) {
	libraryFilename := getDynamicLibraryFilename(target)
	if libraryFilename == "" {
		log.Fatalf("target %s/%s does not have a dynamic library available (use static linking via CGO instead)",
			target.GOOS, target.ARCH)
	}

	directoryName := getLibraryDirectoryName(target)

	// Get the latest commit from the go branch
	goBranchCommit := runCommandOutput(".", "git", "ls-remote", "https://github.com/sagernet/cronet-go.git", "refs/heads/go")
	if goBranchCommit == "" {
		log.Fatal("failed to get go branch commit")
	}
	// Output format: "<commit>\trefs/heads/go\n"
	commitHash := strings.Fields(goBranchCommit)[0]

	modulePath := fmt.Sprintf("github.com/sagernet/cronet-go/lib/%s@%s", directoryName, commitHash)

	log.Printf("Downloading module %s...", modulePath)

	output := runCommandOutput(".", "go", "mod", "download", "-json", modulePath)

	var moduleInfo struct {
		Dir string `json:"Dir"`
	}
	if err := json.Unmarshal([]byte(output), &moduleInfo); err != nil {
		log.Fatalf("failed to parse module info: %v", err)
	}

	if moduleInfo.Dir == "" {
		log.Fatalf("module directory not found in download output")
	}

	sourcePath := filepath.Join(moduleInfo.Dir, libraryFilename)
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		log.Fatalf("library file not found: %s", sourcePath)
	}

	destinationFilename := libraryFilename
	if outputName != "" {
		destinationFilename = outputName
	}
	destinationPath := filepath.Join(outputDirectory, destinationFilename)
	copyFile(sourcePath, destinationPath)

	log.Printf("Extracted %s to %s", destinationFilename, outputDirectory)
}

func getDynamicLibraryFilename(target Target) string {
	switch target.GOOS {
	case "windows":
		return "libcronet.dll"
	case "linux":
		if target.Libc == "musl" {
			return "" // musl builds are static only
		}
		return "libcronet.so"
	default:
		return "" // macOS, iOS, tvOS, Android use static libraries
	}
}
