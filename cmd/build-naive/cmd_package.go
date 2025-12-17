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

var (
	commandPackage = &cobra.Command{
		Use:   "package",
		Short: "Package libraries and generate CGO config files",
		Run: func(cmd *cobra.Command, args []string) {
			targets := parseTargets()
			packageTargets(targets)
		},
	}
	localMode bool
)

func init() {
	mainCommand.AddCommand(commandPackage)
	commandPackage.Flags().BoolVar(&localMode, "local", false, "Generate CGO files in main module for local testing")
}

func packageTargets(targets []Target) {
	log.Printf("Packaging libraries for %d target(s)", len(targets))

	libraryDirectory := filepath.Join(projectRoot, "lib")
	includeDirectory := filepath.Join(projectRoot, "include")

	os.RemoveAll(libraryDirectory)
	os.RemoveAll(includeDirectory)
	os.MkdirAll(includeDirectory, 0o755)

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

	for _, t := range targets {
		targetDirectory := filepath.Join(libraryDirectory, getLibraryDirectoryName(t))
		os.MkdirAll(targetDirectory, 0o755)

		outputDirectory := getOutputDirectory(t)

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
			sourceStatic := filepath.Join(srcRoot, outputDirectory, "obj/components/cronet/libcronet_static.a")
			destinationStatic := filepath.Join(targetDirectory, "libcronet.a")
			if _, err := os.Stat(sourceStatic); os.IsNotExist(err) {
				log.Printf("Warning: static library not found for %s, skipping", formatTargetLog(t))
			} else {
				copyFile(sourceStatic, destinationStatic)
				log.Printf("Copied static library for %s", formatTargetLog(t))
			}

			// For Linux glibc, also copy shared library (for purego mode)
			if t.GOOS == "linux" && t.Libc != "musl" {
				sourceShared := filepath.Join(srcRoot, outputDirectory, "libcronet.so")
				destinationShared := filepath.Join(targetDirectory, "libcronet.so")
				if _, err := os.Stat(sourceShared); err == nil {
					copyFile(sourceShared, destinationShared)
					log.Printf("Copied shared library for %s", formatTargetLog(t))
				}
			}
		}
	}

	generateCGOConfig()
	if localMode {
		generateLocalCGOFiles(targets)
	} else {
		generateSubmodules(targets)
	}

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

func generateLocalCGOFiles(targets []Target) {
	for _, t := range targets {
		if t.GOOS == "windows" {
			log.Printf("Skipping local CGO file for %s (static linking not supported)", formatTargetLog(t))
			continue
		}

		directoryName := getLibraryDirectoryName(t)
		outputDirectory := getOutputDirectory(t)

		linkFlags, err := extractLinkFlags(outputDirectory)
		if err != nil {
			log.Fatalf("failed to extract link flags for %s: %v", formatTargetLog(t), err)
		}

		buildTag := getBuildTag(t)

		var ldFlags []string

		libraryPath := fmt.Sprintf("${SRCDIR}/lib/%s/libcronet.a", directoryName)
		if t.GOOS == "darwin" || t.GOOS == "ios" {
			ldFlags = append(ldFlags, libraryPath)
		} else {
			ldFlags = append(ldFlags, fmt.Sprintf("-L${SRCDIR}/lib/%s", directoryName), "-l:libcronet.a")
		}

		ldFlags = append(ldFlags, linkFlags.LDFlags...)
		ldFlags = append(ldFlags, linkFlags.Libs...)
		ldFlags = append(ldFlags, linkFlags.Frameworks...)

		if t.GOOS == "linux" && t.Libc == "musl" {
			ldFlags = append(ldFlags, "-static")
		}

		cgoContent := fmt.Sprintf(`//go:build %s && !with_purego

package cronet

// #cgo LDFLAGS: %s
import "C"
`, buildTag, strings.Join(ldFlags, " "))

		fileName := fmt.Sprintf("lib_%s_cgo.go", directoryName)
		cgoPath := filepath.Join(projectRoot, fileName)
		err = os.WriteFile(cgoPath, []byte(cgoContent), 0o644)
		if err != nil {
			log.Fatalf("failed to write %s: %v", fileName, err)
		}

		log.Printf("Generated %s", fileName)
	}
}

func getLibraryDirectoryName(t Target) string {
	osName := t.GOOS
	if t.Platform == "tvos" {
		osName = "tvos"
	}

	name := fmt.Sprintf("%s_%s", osName, t.ARCH)

	if t.Environment == "simulator" {
		name += "_simulator"
	}

	if t.Libc == "musl" {
		name += "_musl"
	}

	return name
}

func getBuildTag(t Target) string {
	// iOS/tvOS use gomobile-compatible tags
	if t.GOOS == "ios" {
		parts := []string{"ios", t.ARCH}

		if t.Platform == "tvos" {
			parts = append(parts, "tvos")
			if t.Environment == "simulator" {
				parts = append(parts, "tvossimulator")
			} else {
				parts = append(parts, "!tvossimulator")
			}
		} else {
			parts = append(parts, "!tvos")
			if t.Environment == "simulator" {
				parts = append(parts, "iossimulator")
			} else {
				parts = append(parts, "!iossimulator")
			}
		}

		return strings.Join(parts, " && ")
	}

	if t.Libc == "musl" {
		return fmt.Sprintf("%s && !android && %s && with_musl", t.GOOS, t.ARCH)
	}
	if t.GOOS == "linux" {
		return fmt.Sprintf("%s && !android && %s && !with_musl", t.GOOS, t.ARCH)
	}
	if t.GOOS == "darwin" {
		return fmt.Sprintf("%s && !ios && %s", t.GOOS, t.ARCH)
	}
	return fmt.Sprintf("%s && %s", t.GOOS, t.ARCH)
}

type LinkFlags struct {
	Libs       []string
	Frameworks []string
	LDFlags    []string
}

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

		goModContent := fmt.Sprintf(`module github.com/sagernet/cronet-go/lib/%s

go 1.20
`, directoryName)
		goModPath := filepath.Join(targetDirectory, "go.mod")
		err := os.WriteFile(goModPath, []byte(goModContent), 0o644)
		if err != nil {
			log.Fatalf("failed to write go.mod: %v", err)
		}

		if t.GOOS == "windows" {
			// Windows: only generate purego mode files (version constant only)
			// Static linking is not supported (Chromium uses MSVC, Go CGO only supports MinGW)
			// DLL is copied to lib directory for downstream extraction, must be distributed alongside binary
			generateWindowsPuregoFile(targetDirectory, packageName, chromiumVersion)
			runCommand(targetDirectory, "go", "mod", "tidy")
			log.Printf("Generated submodule lib/%s (purego only)", directoryName)
			continue
		}

		outputDirectory := getOutputDirectory(t)
		linkFlags, err := extractLinkFlags(outputDirectory)
		if err != nil {
			log.Fatalf("failed to extract link flags for %s: %v", formatTargetLog(t), err)
		}

		log.Printf("Extracted link flags for %s: libs=%v frameworks=%v ldflags=%v", formatTargetLog(t), linkFlags.Libs, linkFlags.Frameworks, linkFlags.LDFlags)

		buildTag := getBuildTag(t)

		var ldFlags []string

		if t.GOOS == "darwin" || t.GOOS == "ios" {
			ldFlags = append(ldFlags, "${SRCDIR}/libcronet.a")
		} else {
			ldFlags = append(ldFlags, "-L${SRCDIR}", "-l:libcronet.a")
		}

		ldFlags = append(ldFlags, linkFlags.LDFlags...)
		ldFlags = append(ldFlags, linkFlags.Libs...)
		ldFlags = append(ldFlags, linkFlags.Frameworks...)

		if t.GOOS == "linux" && t.Libc == "musl" {
			ldFlags = append(ldFlags, "-static")
		}

		cgoContent := fmt.Sprintf(`//go:build %s && !with_purego

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

		// Generate purego stub file for non-Windows platforms
		// This allows the package to compile in purego mode (user must provide .so/.dylib)
		generatePuregoStubFile(targetDirectory, packageName, chromiumVersion)

		runCommand(targetDirectory, "go", "mod", "tidy")

		log.Printf("Generated submodule lib/%s", directoryName)
	}
}

func generatePuregoStubFile(targetDirectory, packageName, chromiumVersion string) {
	content := fmt.Sprintf(`//go:build with_purego

package %s

const Version = "%s"
`, packageName, chromiumVersion)

	stubPath := filepath.Join(targetDirectory, "libcronet_purego.go")
	err := os.WriteFile(stubPath, []byte(content), 0o644)
	if err != nil {
		log.Fatalf("failed to write libcronet_purego.go: %v", err)
	}
	log.Printf("Generated libcronet_purego.go for %s", packageName)
}

func generateWindowsPuregoFile(targetDirectory, packageName, chromiumVersion string) {
	// Windows: generate a simple version file (DLL must be distributed alongside the binary)
	// The DLL file is still copied to the lib directory for downstream extraction
	content := fmt.Sprintf(`//go:build with_purego

package %s

const Version = "%s"
`, packageName, chromiumVersion)

	filePath := filepath.Join(targetDirectory, "libcronet.go")
	err := os.WriteFile(filePath, []byte(content), 0o644)
	if err != nil {
		log.Fatalf("failed to write libcronet.go: %v", err)
	}
	log.Printf("Generated libcronet.go for %s (version only)", packageName)
}
