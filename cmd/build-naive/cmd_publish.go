package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var publishBranch string

var commandPublish = &cobra.Command{
	Use:   "publish",
	Short: "Commit to go branch and push",
	Run: func(cmd *cobra.Command, args []string) {
		publish()
	},
}

func init() {
	commandPublish.Flags().StringVar(&publishBranch, "branch", "go", "Target branch to publish to")
	mainCommand.AddCommand(commandPublish)
}

func publish() {
	log.Printf("Publishing to %s branch...", publishBranch)

	mainCommit := strings.TrimSpace(runCommandOutput(projectRoot, "git", "rev-parse", "HEAD"))

	temporaryDirectory, err := os.MkdirTemp("", "cronet-go-publish-")
	if err != nil {
		log.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		runCommand(projectRoot, "git", "worktree", "remove", "--force", temporaryDirectory)
		os.RemoveAll(temporaryDirectory)
	}()

	runCommand(projectRoot, "git", "worktree", "add", temporaryDirectory, "HEAD")

	// === Step 1: Push main module + lib submodules ===
	log.Print("Step 1: Publishing main module and lib submodules...")

	copyDirectory(filepath.Join(projectRoot, "lib"), filepath.Join(temporaryDirectory, "lib"))
	copyDirectory(filepath.Join(projectRoot, "include"), filepath.Join(temporaryDirectory, "include"))
	copyFile(filepath.Join(projectRoot, "include_cgo.go"), filepath.Join(temporaryDirectory, "include_cgo.go"))

	// Use -f (force add) to include .gitignore'd files
	runCommand(temporaryDirectory, "git", "add", "-f", "-A")
	commitMessage := fmt.Sprintf("Build from %s", mainCommit[:8])
	runCommand(temporaryDirectory, "git", "commit", "-m", commitMessage)

	runCommand(temporaryDirectory, "git", "push", "-f", "origin", "HEAD:refs/heads/"+publishBranch)

	firstCommit := strings.TrimSpace(runCommandOutput(temporaryDirectory, "git", "rev-parse", "HEAD"))
	commitTime := getCommitTime(temporaryDirectory, firstCommit)
	pseudoVersion := formatPseudoVersion(commitTime, firstCommit)

	log.Printf("First commit: %s, pseudo-version: %s", firstCommit[:12], pseudoVersion)

	// === Step 2: Generate and push all package ===
	log.Print("Step 2: Generating all package...")

	libDirectory := filepath.Join(temporaryDirectory, "lib")
	libEntries, err := os.ReadDir(libDirectory)
	if err != nil {
		log.Fatalf("failed to read lib directory: %v", err)
	}

	var builtTargets []string
	for _, entry := range libEntries {
		if entry.IsDir() {
			builtTargets = append(builtTargets, entry.Name())
		}
	}

	if len(builtTargets) == 0 {
		log.Fatal("no lib directories found")
	}

	generateAllPackage(temporaryDirectory, pseudoVersion, builtTargets)

	// Fix lib submodules' go.mod to use correct pseudo-version
	// (package stage's go mod tidy may have selected wrong version)
	fixLibSubmoduleVersions(filepath.Join(temporaryDirectory, "lib"), builtTargets, pseudoVersion)

	// Use GOPROXY=direct to avoid proxy caching issues when using the new pseudo-version
	runGoModTidy(filepath.Join(temporaryDirectory, "all"))

	// Force correct version after tidy (tidy may select a higher tagged version due to MVS)
	forceMainModuleVersion(filepath.Join(temporaryDirectory, "all"), pseudoVersion)

	runCommand(temporaryDirectory, "git", "add", "-f", "-A")
	runCommand(temporaryDirectory, "git", "commit", "-m", "Generate all package")
	runCommand(temporaryDirectory, "git", "push", "origin", "HEAD:"+publishBranch)

	log.Printf("Published to %s branch!", publishBranch)
}

func formatPseudoVersion(commitTime time.Time, commitHash string) string {
	timestamp := commitTime.UTC().Format("20060102150405")
	return fmt.Sprintf("v0.0.0-%s-%s", timestamp, commitHash[:12])
}

func getCommitTime(directory, commitHash string) time.Time {
	output := runCommandOutput(directory, "git", "show", "-s", "--format=%cI", commitHash)
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(output))
	if err != nil {
		log.Fatalf("failed to parse commit time: %v", err)
	}
	return t
}

func generateAllPackage(directory, pseudoVersion string, builtTargets []string) {
	allDirectory := filepath.Join(directory, "all")
	err := os.MkdirAll(allDirectory, 0o755)
	if err != nil {
		log.Fatalf("failed to create all directory: %v", err)
	}

	generateAllGoMod(allDirectory, pseudoVersion, builtTargets)

	for _, targetName := range builtTargets {
		generatePlatformImportFile(allDirectory, targetName)
	}

	log.Printf("Generated all package with %d platforms", len(builtTargets))
}

func generateAllGoMod(allDirectory, pseudoVersion string, builtTargets []string) {
	var builder strings.Builder
	builder.WriteString("module github.com/sagernet/cronet-go/all\n\n")
	builder.WriteString("go 1.20\n\n")
	builder.WriteString("require (\n")
	builder.WriteString(fmt.Sprintf("\tgithub.com/sagernet/cronet-go %s\n", pseudoVersion))
	for _, targetName := range builtTargets {
		builder.WriteString(fmt.Sprintf("\tgithub.com/sagernet/cronet-go/lib/%s %s\n", targetName, pseudoVersion))
	}
	builder.WriteString(")\n")

	goModPath := filepath.Join(allDirectory, "go.mod")
	err := os.WriteFile(goModPath, []byte(builder.String()), 0o644)
	if err != nil {
		log.Fatalf("failed to write go.mod: %v", err)
	}
}

func generatePlatformImportFile(allDirectory, targetName string) {
	buildTag := getBuildTagForTarget(targetName)
	packageName := strings.ReplaceAll(targetName, "-", "_")

	content := fmt.Sprintf(`//go:build %s

package all

import (
	_ "github.com/sagernet/cronet-go"
	_ "github.com/sagernet/cronet-go/lib/%s"
)
`, buildTag, targetName)

	fileName := packageName + ".go"
	filePath := filepath.Join(allDirectory, fileName)
	err := os.WriteFile(filePath, []byte(content), 0o644)
	if err != nil {
		log.Fatalf("failed to write %s: %v", fileName, err)
	}
}

// getBuildTagForTarget returns the build tag for a given target directory name.
// Directory names follow the pattern: {platform}_{goarch}[_simulator][_musl]
// where platform is goos (linux, darwin, windows, android) or tvos/ios for Apple platforms.
func getBuildTagForTarget(targetName string) string {
	parts := strings.Split(targetName, "_")
	if len(parts) < 2 {
		log.Fatalf("invalid target name: %s", targetName)
	}

	goos := parts[0]
	goarch := parts[1]

	isSimulator := false
	isMusl := false
	isTvOS := false

	for i := 2; i < len(parts); i++ {
		switch parts[i] {
		case "simulator":
			isSimulator = true
		case "musl":
			isMusl = true
		}
	}

	// Handle tvOS: directory prefix is "tvos" but GOOS is "ios"
	if goos == "tvos" {
		isTvOS = true
		goos = "ios"
	}

	// Handle iOS/tvOS with gomobile-compatible tags
	if goos == "ios" {
		tagParts := []string{"ios", goarch}

		if isTvOS {
			tagParts = append(tagParts, "tvos")
			if isSimulator {
				tagParts = append(tagParts, "tvossimulator")
			} else {
				tagParts = append(tagParts, "!tvossimulator")
			}
		} else {
			tagParts = append(tagParts, "!tvos")
			if isSimulator {
				tagParts = append(tagParts, "iossimulator")
			} else {
				tagParts = append(tagParts, "!iossimulator")
			}
		}

		return strings.Join(tagParts, " && ")
	}

	if isMusl {
		return fmt.Sprintf("%s && !android && %s && with_musl", goos, goarch)
	}

	if goos == "linux" {
		return fmt.Sprintf("%s && !android && %s && !with_musl", goos, goarch)
	}

	if goos == "darwin" {
		return fmt.Sprintf("%s && !ios && %s", goos, goarch)
	}

	// Windows: purego only
	if goos == "windows" {
		return fmt.Sprintf("%s && %s && with_purego", goos, goarch)
	}

	return fmt.Sprintf("%s && %s", goos, goarch)
}

func runGoModTidy(directory string) {
	log.Printf("Running go mod tidy in %s with GOPROXY=direct...", directory)
	command := exec.Command("go", "mod", "tidy")
	command.Dir = directory
	command.Env = append(os.Environ(), "GOPROXY=direct", "GOSUMDB=off")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err := command.Run()
	if err != nil {
		log.Fatalf("go mod tidy failed: %v", err)
	}
}

func forceMainModuleVersion(directory, version string) {
	log.Printf("Forcing main module version to %s...", version)
	runCommand(directory, "go", "mod", "edit", "-require=github.com/sagernet/cronet-go@"+version)
}

func fixLibSubmoduleVersions(libDirectory string, targets []string, version string) {
	log.Printf("Fixing lib submodule versions to %s...", version)
	for _, targetName := range targets {
		submoduleDirectory := filepath.Join(libDirectory, targetName)
		goModPath := filepath.Join(submoduleDirectory, "go.mod")

		// Check if go.mod exists
		if _, err := os.Stat(goModPath); os.IsNotExist(err) {
			continue
		}

		// Check if this submodule depends on main module
		content, err := os.ReadFile(goModPath)
		if err != nil {
			log.Fatalf("failed to read %s: %v", goModPath, err)
		}

		if !strings.Contains(string(content), "github.com/sagernet/cronet-go") {
			continue
		}

		// Force correct version
		runCommand(submoduleDirectory, "go", "mod", "edit", "-require=github.com/sagernet/cronet-go@"+version)
		log.Printf("  Fixed lib/%s", targetName)
	}
}
