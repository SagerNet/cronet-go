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

	// Create lib directories
	libraryDirectory := filepath.Join(projectRoot, "lib")
	includeDirectory := filepath.Join(projectRoot, "include")

	os.RemoveAll(libraryDirectory)
	os.RemoveAll(includeDirectory)
	os.MkdirAll(includeDirectory, 0o755)

	// Copy headers
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
		targetDirectory := filepath.Join(libraryDirectory, fmt.Sprintf("%s_%s", t.GOOS, t.ARCH))
		os.MkdirAll(targetDirectory, 0o755)

		outputDirectory := fmt.Sprintf("out/cronet-%s-%s", t.OS, t.CPU)

		// Copy static library
		sourceStatic := filepath.Join(srcRoot, outputDirectory, "obj/components/cronet/libcronet_static.a")
		destinationStatic := filepath.Join(targetDirectory, "libcronet.a")
		if _, err := os.Stat(sourceStatic); os.IsNotExist(err) {
			log.Printf("Warning: static library not found for %s/%s, skipping", t.GOOS, t.ARCH)
		} else {
			copyFile(sourceStatic, destinationStatic)
			log.Printf("Copied static library for %s/%s", t.GOOS, t.ARCH)
		}
	}

	// Generate CGO config files
	generateCGOConfigs(targets)

	log.Print("Package complete!")
}

func generateCGOConfigs(targets []Target) {
	for _, t := range targets {
		libraryDirectory := "${SRCDIR}/lib/" + t.GOOS + "_" + t.ARCH

		// Platform-specific flags
		var platformFlags []string
		switch t.GOOS {
		case "linux":
			platformFlags = []string{"-ldl", "-lpthread", "-lm", "-lresolv"}
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

		// Generate static library config
		// macOS: use direct path (doesn't support -l: syntax), also needs IOKit and libbsm
		// Linux: use -l:libcronet.a syntax
		staticFilename := fmt.Sprintf("cgo_%s_%s.go", t.GOOS, t.ARCH)
		staticPath := filepath.Join(projectRoot, staticFilename)
		var staticFlags []string
		if t.GOOS == "darwin" {
			staticFlags = append([]string{libraryDirectory + "/libcronet.a", "-lc++", "-lbsm", "-framework IOKit"}, platformFlags...)
		} else if t.GOOS == "ios" {
			staticFlags = append([]string{libraryDirectory + "/libcronet.a", "-lc++"}, platformFlags...)
		} else {
			staticFlags = append([]string{"-L" + libraryDirectory, "-l:libcronet.a", "-lc++"}, platformFlags...)
		}
		staticContent := fmt.Sprintf(`//go:build %s && %s

package cronet

// #cgo CFLAGS: -I${SRCDIR}/include
// #cgo LDFLAGS: %s
import "C"
`, t.GOOS, t.ARCH, strings.Join(staticFlags, " "))

		err := os.WriteFile(staticPath, []byte(staticContent), 0o644)
		if err != nil {
			log.Fatalf("failed to write %s: %v", staticFilename, err)
		}
		log.Printf("Generated %s", staticFilename)
	}
}
