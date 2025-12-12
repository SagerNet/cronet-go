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

	// Copy libraries for each target
	for _, t := range targets {
		targetDirectory := filepath.Join(libraryDirectory, getLibraryDirectoryName(t))
		os.MkdirAll(targetDirectory, 0o755)

		outputDirectory := fmt.Sprintf("out/cronet-%s-%s", t.OS, t.CPU)

		if t.GOOS == "windows" {
			// Windows: only copy DLL (static linking not supported - Chromium uses MSVC, Go CGO only supports MinGW)
			sourceDLL := filepath.Join(srcRoot, outputDirectory, "cronet.dll")
			destinationDLL := filepath.Join(targetDirectory, "libcronet.dll")
			if _, err := os.Stat(sourceDLL); os.IsNotExist(err) {
				log.Printf("Warning: DLL not found for %s/%s, skipping", t.GOOS, t.ARCH)
			} else {
				copyFile(sourceDLL, destinationDLL)
				log.Printf("Copied DLL for %s/%s", t.GOOS, t.ARCH)
			}
		} else {
			// Other platforms: copy static library
			sourceStatic := filepath.Join(srcRoot, outputDirectory, "obj/components/cronet/libcronet_static.a")
			destinationStatic := filepath.Join(targetDirectory, "libcronet.a")
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

			// For Linux glibc, also copy shared library (for testing and release, not for go module)
			if t.GOOS == "linux" && t.Libc != "musl" {
				sourceShared := filepath.Join(srcRoot, outputDirectory, "libcronet.so")
				destinationShared := filepath.Join(targetDirectory, "libcronet.so")
				if _, err := os.Stat(sourceShared); err == nil {
					copyFile(sourceShared, destinationShared)
					log.Printf("Copied shared library for %s/%s", t.GOOS, t.ARCH)
				}
			}
		}
	}

	// Generate main module cgo.go and submodule files
	generateCGOConfig()
	generateSubmodules(targets)

	log.Print("Package complete!")
}

func generateCGOConfig() {
	content := `//go:build !with_purego

package cronet

// #cgo CFLAGS: -I${SRCDIR}/include
import "C"
`
	path := filepath.Join(projectRoot, "include_cgo.go")
	err := os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		log.Fatalf("failed to write include_cgo.go: %v", err)
	}
	log.Print("Generated include_cgo.go")
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
	LDFlags    []string // Linker flags (e.g., -Wl,-wrap,realpath)
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
	ldflagsRegex := regexp.MustCompile(`^\s*ldflags\s*=\s*(.*)$`)

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

		if matches := ldflagsRegex.FindStringSubmatch(line); matches != nil {
			ldflagsStr := strings.TrimSpace(matches[1])
			if ldflagsStr != "" {
				flags.LDFlags = parseLDFlags(ldflagsStr)
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

// parseLDFlags extracts relevant linker flags from the ldflags string.
// We specifically extract -Wl,-wrap,* flags needed for Android allocator shim.
func parseLDFlags(input string) []string {
	var result []string
	for _, flag := range strings.Fields(input) {
		// Extract -Wl,-wrap,* flags needed for Android allocator shim
		if strings.HasPrefix(flag, "-Wl,-wrap,") {
			result = append(result, flag)
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
		packageName := strings.ReplaceAll(directoryName, "-", "_")

		// Generate go.mod
		goModContent := fmt.Sprintf(`module github.com/sagernet/cronet-go/lib/%s

go 1.20
`, directoryName)
		goModPath := filepath.Join(targetDirectory, "go.mod")
		err := os.WriteFile(goModPath, []byte(goModContent), 0o644)
		if err != nil {
			log.Fatalf("failed to write go.mod: %v", err)
		}

		if t.GOOS == "windows" {
			// Windows: only generate purego mode files (DLL embed)
			// Static linking is not supported (Chromium uses MSVC, Go CGO only supports MinGW)
			generateEmbedFile(targetDirectory, packageName, chromiumVersion)
			runCommand(targetDirectory, "go", "mod", "tidy")
			log.Printf("Generated submodule lib/%s (purego only)", directoryName)
			continue
		}

		// Extract linking flags from ninja file
		outputDirectory := fmt.Sprintf("out/cronet-%s-%s", t.OS, t.CPU)
		linkFlags, err := extractLinkFlags(outputDirectory)
		if err != nil {
			log.Fatalf("failed to extract link flags for %s/%s: %v", t.GOOS, t.ARCH, err)
		}

		log.Printf("Extracted link flags for %s/%s: libs=%v frameworks=%v ldflags=%v", t.GOOS, t.ARCH, linkFlags.Libs, linkFlags.Frameworks, linkFlags.LDFlags)

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

		// Generate libcronet_cgo.go with CGO config
		var ldFlags []string

		// Add static library reference
		if t.GOOS == "darwin" || t.GOOS == "ios" {
			ldFlags = append(ldFlags, "${SRCDIR}/libcronet.a")
		} else {
			ldFlags = append(ldFlags, "-L${SRCDIR}", "-l:libcronet.a")
		}

		// Add extracted ldflags (e.g., -Wl,-wrap,* for Android)
		ldFlags = append(ldFlags, linkFlags.LDFlags...)

		// Add extracted libs
		ldFlags = append(ldFlags, linkFlags.Libs...)

		// Add extracted frameworks
		ldFlags = append(ldFlags, linkFlags.Frameworks...)

		// Add Linux-specific flags
		if t.GOOS == "linux" && t.Libc == "musl" {
			ldFlags = append(ldFlags, "-static")
		}

		cgoContent := fmt.Sprintf(`//go:build %s

package %s

// #cgo LDFLAGS: %s
import "C"

const Version = "%s"
`, buildTag, packageName, strings.Join(ldFlags, " "), chromiumVersion)

		cgoPath := filepath.Join(targetDirectory, "libcronet_cgo.go")
		err = os.WriteFile(cgoPath, []byte(cgoContent), 0o644)
		if err != nil {
			log.Fatalf("failed to write libcronet_cgo.go: %v", err)
		}

		// Run go mod tidy
		runCommand(targetDirectory, "go", "mod", "tidy")

		log.Printf("Generated submodule lib/%s", directoryName)
	}
}

// generateEmbedFile generates libcronet.go for Windows targets.
// This allows the DLL to be embedded in the binary for purego mode.
func generateEmbedFile(targetDirectory, packageName, chromiumVersion string) {
	content := fmt.Sprintf(`//go:build with_purego

package %s

import (
	_ "embed"
)

//go:embed libcronet.dll
var EmbeddedDLL []byte

const EmbeddedVersion = "%s"
`, packageName, chromiumVersion)

	embedPath := filepath.Join(targetDirectory, "libcronet.go")
	err := os.WriteFile(embedPath, []byte(content), 0o644)
	if err != nil {
		log.Fatalf("failed to write libcronet.go: %v", err)
	}
	log.Printf("Generated libcronet.go for %s", packageName)
}
