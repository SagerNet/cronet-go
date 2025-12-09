package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
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

// LinkFlags contains linking parameters extracted from ninja build files
type LinkFlags struct {
	Libs       []string // System libraries (e.g., -ldl, -lpthread)
	Frameworks []string // macOS/iOS frameworks (e.g., -framework Security)
}

// extractLinkFlags parses the ninja file for cronet_sample to extract linking parameters
func extractLinkFlags(outputDirectory string) (LinkFlags, error) {
	ninjaPath := filepath.Join(srcRoot, outputDirectory, "obj/components/cronet/cronet_sample.ninja")
	file, err := os.Open(ninjaPath)
	if err != nil {
		return LinkFlags{}, fmt.Errorf("failed to open ninja file %s: %w", ninjaPath, err)
	}
	defer file.Close()

	var flags LinkFlags
	libsRegex := regexp.MustCompile(`^\s*libs\s*=\s*(.*)$`)
	frameworksRegex := regexp.MustCompile(`^\s*frameworks\s*=\s*(.*)$`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if matches := libsRegex.FindStringSubmatch(line); matches != nil {
			libsStr := strings.TrimSpace(matches[1])
			if libsStr != "" {
				for _, lib := range strings.Fields(libsStr) {
					// Filter out linker scripts (.lds) - they're not needed for
					// static linking and Go's CGO rejects them as invalid flags
					if !strings.HasSuffix(lib, ".lds") {
						flags.Libs = append(flags.Libs, lib)
					}
				}
			}
		}

		if matches := frameworksRegex.FindStringSubmatch(line); matches != nil {
			frameworksStr := strings.TrimSpace(matches[1])
			if frameworksStr != "" {
				flags.Frameworks = parseFrameworks(frameworksStr)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return LinkFlags{}, fmt.Errorf("failed to read ninja file: %w", err)
	}

	return flags, nil
}

// parseFrameworks parses "-framework Foo -framework Bar" into []string{"-framework Foo", "-framework Bar"}
func parseFrameworks(input string) []string {
	var result []string
	parts := strings.Fields(input)
	for i := 0; i < len(parts); i++ {
		if parts[i] == "-framework" && i+1 < len(parts) {
			result = append(result, "-framework "+parts[i+1])
			i++
		}
	}
	return result
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

		// Extract linking flags from ninja file
		outputDirectory := fmt.Sprintf("out/cronet-%s-%s", t.OS, t.CPU)
		linkFlags, err := extractLinkFlags(outputDirectory)
		if err != nil {
			log.Fatalf("failed to extract link flags for %s/%s: %v", t.GOOS, t.ARCH, err)
		}

		log.Printf("Extracted link flags for %s/%s: libs=%v frameworks=%v", t.GOOS, t.ARCH, linkFlags.Libs, linkFlags.Frameworks)

		// Build tags
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

		// Build LDFLAGS from extracted values
		var ldFlags []string

		// Add static library reference
		if t.GOOS == "windows" {
			ldFlags = append(ldFlags, "${SRCDIR}/cronet.lib")
		} else if t.GOOS == "darwin" || t.GOOS == "ios" {
			ldFlags = append(ldFlags, "${SRCDIR}/libcronet.a")
		} else {
			ldFlags = append(ldFlags, "-L${SRCDIR}", "-l:libcronet.a")
		}

		// Add libc++ for C++ runtime on Darwin platforms
		if t.GOOS == "darwin" || t.GOOS == "ios" {
			ldFlags = append(ldFlags, "-lc++")
		}

		// Add extracted libs
		ldFlags = append(ldFlags, linkFlags.Libs...)

		// Add extracted frameworks
		ldFlags = append(ldFlags, linkFlags.Frameworks...)

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
