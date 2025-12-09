package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var commandPackage = &cobra.Command{
	Use:   "package",
	Short: "Package libraries and generate CGO config files",
	Run: func(cmd *cobra.Command, args []string) {
		targets := parseTargets()
		packageTargets(targets)
	},
}

func init() {
	mainCommand.AddCommand(commandPackage)
}

func packageTargets(targets []Target) {
	log.Printf("Packaging libraries for %d target(s)", len(targets))

	// Create lib directory
	libraryDirectory := filepath.Join(projectRoot, "lib")
	includeDirectory := filepath.Join(projectRoot, "include")

	os.RemoveAll(libraryDirectory)
	os.RemoveAll(includeDirectory)
	os.MkdirAll(includeDirectory, 0o755)

	// Copy headers to main module
	headers := []struct {
		source      string
		destination string
	}{
		{filepath.Join(srcRoot, "components/cronet/native/include/cronet_c.h"), "cronet_c.h"},
		{filepath.Join(srcRoot, "components/cronet/native/include/cronet_export.h"), "cronet_export.h"},
		{filepath.Join(srcRoot, "components/cronet/native/generated/cronet.idl_c.h"), "cronet.idl_c.h"},
		{filepath.Join(srcRoot, "components/grpc_support/include/bidirectional_stream_c.h"), "bidirectional_stream_c.h"},
	}

	for _, header := range headers {
		copyFile(header.source, filepath.Join(includeDirectory, header.destination))
	}
	log.Print("Copied headers to include/")

	// Copy static libraries for each target
	for _, t := range targets {
		targetDirectory := filepath.Join(libraryDirectory, getLibraryDirectoryName(t))
		os.MkdirAll(targetDirectory, 0o755)

		outputDirectory := fmt.Sprintf("out/cronet-%s-%s", t.OS, t.CPU)

		// Copy static library
		var sourceStatic, destinationStatic string
		if t.GOOS == "windows" {
			sourceStatic = filepath.Join(srcRoot, outputDirectory, "obj/components/cronet/cronet_static.lib")
			destinationStatic = filepath.Join(targetDirectory, "cronet.lib")
		} else {
			sourceStatic = filepath.Join(srcRoot, outputDirectory, "obj/components/cronet/libcronet_static.a")
			destinationStatic = filepath.Join(targetDirectory, "libcronet.a")
		}
		if _, err := os.Stat(sourceStatic); os.IsNotExist(err) {
			log.Printf("Warning: static library not found for %s/%s, skipping", t.GOOS, t.ARCH)
		} else {
			copyFile(sourceStatic, destinationStatic)
			if t.Libc == "musl" {
				log.Printf("Copied static library for %s/%s (musl)", t.GOOS, t.ARCH)
			} else {
				log.Printf("Copied static library for %s/%s", t.GOOS, t.ARCH)
			}
		}
	}

	// Generate main module cgo.go and submodule files
	generateCGOConfig()
	generateSubmodules(targets)

	log.Print("Package complete!")
}

func generateCGOConfig() {
	content := `package cronet

// #cgo CFLAGS: -I${SRCDIR}/include
import "C"
`
	path := filepath.Join(projectRoot, "include.go")
	err := os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		log.Fatalf("failed to write include.go: %v", err)
	}
	log.Print("Generated include.go")
}

// getLibraryDirectoryName returns the library directory name for a target
func getLibraryDirectoryName(t Target) string {
	if t.Libc == "musl" {
		return fmt.Sprintf("%s_%s_musl", t.GOOS, t.ARCH)
	}
	return fmt.Sprintf("%s_%s", t.GOOS, t.ARCH)
}

func generateSubmodules(targets []Target) {
	versionFile := filepath.Join(naiveRoot, "CHROMIUM_VERSION")
	versionData, err := os.ReadFile(versionFile)
	if err != nil {
		log.Fatalf("failed to read CHROMIUM_VERSION: %v", err)
	}
	chromiumVersion := strings.TrimSpace(string(versionData))

	for _, t := range targets {
		directoryName := getLibraryDirectoryName(t)
		targetDirectory := filepath.Join(projectRoot, "lib", directoryName)

		// Generate go.mod
		goModContent := fmt.Sprintf(`module github.com/sagernet/cronet-go/lib/%s

go 1.20
`, directoryName)
		goModPath := filepath.Join(targetDirectory, "go.mod")
		err := os.WriteFile(goModPath, []byte(goModContent), 0o644)
		if err != nil {
			log.Fatalf("failed to write go.mod: %v", err)
		}

		// Platform-specific flags
		var platformFlags []string
		switch t.GOOS {
		case "linux":
			if t.Libc == "musl" {
				platformFlags = []string{}
			} else {
				platformFlags = []string{"-ldl", "-lpthread", "-lm", "-lresolv"}
			}
		case "darwin":
			platformFlags = []string{
				"-framework Security",
				"-framework CoreFoundation",
				"-framework SystemConfiguration",
				"-framework Network",
				"-framework AppKit",
				"-framework CFNetwork",
				"-framework UniformTypeIdentifiers",
			}
		case "windows":
			platformFlags = []string{
				"-lws2_32",
				"-lcrypt32",
				"-lsecur32",
				"-ladvapi32",
				"-lwinhttp",
			}
		case "android":
			platformFlags = []string{"-ldl", "-llog", "-landroid"}
		case "ios":
			platformFlags = []string{
				"-framework Security",
				"-framework CoreFoundation",
				"-framework SystemConfiguration",
				"-framework Network",
				"-framework UIKit",
			}
		}

		// Build tags and LDFLAGS
		var buildTag string
		if t.Libc == "musl" {
			buildTag = fmt.Sprintf("%s && !android && %s && with_musl", t.GOOS, t.ARCH)
		} else if t.GOOS == "linux" {
			buildTag = fmt.Sprintf("%s && !android && %s && !with_musl", t.GOOS, t.ARCH)
		} else if t.GOOS == "darwin" {
			buildTag = fmt.Sprintf("%s && !ios && %s", t.GOOS, t.ARCH)
		} else {
			buildTag = fmt.Sprintf("%s && %s", t.GOOS, t.ARCH)
		}

		var ldFlags []string
		if t.GOOS == "darwin" {
			ldFlags = append([]string{"${SRCDIR}/libcronet.a", "-lc++", "-lbsm", "-framework IOKit"}, platformFlags...)
		} else if t.GOOS == "ios" {
			ldFlags = append([]string{"${SRCDIR}/libcronet.a", "-lc++"}, platformFlags...)
		} else if t.GOOS == "windows" {
			ldFlags = append([]string{"${SRCDIR}/cronet.lib"}, platformFlags...)
		} else if t.Libc == "musl" {
			ldFlags = append([]string{"-L${SRCDIR}", "-l:libcronet.a"}, platformFlags...)
		} else {
			ldFlags = append([]string{"-L${SRCDIR}", "-l:libcronet.a"}, platformFlags...)
		}

		// Generate cgo.go (only LDFLAGS, CFLAGS is in main module)
		packageName := strings.ReplaceAll(directoryName, "-", "_")
		cgoContent := fmt.Sprintf(`//go:build %s

package %s

// #cgo LDFLAGS: %s
import "C"

const Version = "%s"
`, buildTag, packageName, strings.Join(ldFlags, " "), chromiumVersion)

		cgoPath := filepath.Join(targetDirectory, "cgo.go")
		err = os.WriteFile(cgoPath, []byte(cgoContent), 0o644)
		if err != nil {
			log.Fatalf("failed to write cgo.go: %v", err)
		}

		// Run go mod tidy
		runCommand(targetDirectory, "go", "mod", "tidy")

		log.Printf("Generated submodule lib/%s", directoryName)
	}
}
