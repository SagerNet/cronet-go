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

	// Get current commit
	mainCommit := strings.TrimSpace(runCommandOutput(projectRoot, "git", "rev-parse", "HEAD"))

	// Create temp directory for worktree
	temporaryDirectory, err := os.MkdirTemp("", "cronet-go-publish-")
	if err != nil {
		log.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		runCommand(projectRoot, "git", "worktree", "remove", "--force", temporaryDirectory)
		os.RemoveAll(temporaryDirectory)
	}()

	// Create worktree based on current HEAD (keep all files)
	runCommand(projectRoot, "git", "worktree", "add", temporaryDirectory, "HEAD")

	// === Step 1: Push main module + lib submodules ===
	log.Print("Step 1: Publishing main module and lib submodules...")

	// Copy lib, include directories and include_cgo.go
	// Exclude shared libraries (.so) - they are for testing/release only, not for go module
	copyDirectoryExclude(filepath.Join(projectRoot, "lib"), filepath.Join(temporaryDirectory, "lib"), []string{"*.so"})
	copyDirectory(filepath.Join(projectRoot, "include"), filepath.Join(temporaryDirectory, "include"))
	copyFile(filepath.Join(projectRoot, "include_cgo.go"), filepath.Join(temporaryDirectory, "include_cgo.go"))

	// Stage and commit (force add to include .gitignore'd files)
	runCommand(temporaryDirectory, "git", "add", "-f", "-A")
	commitMessage := fmt.Sprintf("Build from %s", mainCommit[:8])
	runCommand(temporaryDirectory, "git", "commit", "-m", commitMessage)

	// Force push to target branch
	runCommand(temporaryDirectory, "git", "push", "-f", "origin", "HEAD:refs/heads/"+publishBranch)

	// Get the commit hash and time of the first push
	firstCommit := strings.TrimSpace(runCommandOutput(temporaryDirectory, "git", "rev-parse", "HEAD"))
	commitTime := getCommitTime(temporaryDirectory, firstCommit)
	pseudoVersion := formatPseudoVersion(commitTime, firstCommit)

	log.Printf("First commit: %s, pseudo-version: %s", firstCommit[:12], pseudoVersion)

	// === Step 2: Generate and push all package ===
	log.Print("Step 2: Generating all package...")

	// Get list of lib directories that were actually built
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

	// Generate all/ package
	generateAllPackage(temporaryDirectory, pseudoVersion, builtTargets)

	// Run go mod tidy with GOPROXY=direct
	runGoModTidy(filepath.Join(temporaryDirectory, "all"))

	// Stage and commit
	runCommand(temporaryDirectory, "git", "add", "-f", "-A")
	runCommand(temporaryDirectory, "git", "commit", "-m", "Generate all package")
	runCommand(temporaryDirectory, "git", "push", "origin", "HEAD:"+publishBranch)

	log.Printf("Published to %s branch!", publishBranch)
}

// formatPseudoVersion generates a Go pseudo-version
// Format: v0.0.0-yyyymmddhhmmss-abcdef123456
func formatPseudoVersion(commitTime time.Time, commitHash string) string {
	timestamp := commitTime.UTC().Format("20060102150405")
	return fmt.Sprintf("v0.0.0-%s-%s", timestamp, commitHash[:12])
}

// getCommitTime retrieves the commit time of a given commit
func getCommitTime(directory, commitHash string) time.Time {
	output := runCommandOutput(directory, "git", "show", "-s", "--format=%cI", commitHash)
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(output))
	if err != nil {
		log.Fatalf("failed to parse commit time: %v", err)
	}
	return t
}

// generateAllPackage generates the all/ aggregation package
func generateAllPackage(directory, pseudoVersion string, builtTargets []string) {
	allDirectory := filepath.Join(directory, "all")
	err := os.MkdirAll(allDirectory, 0o755)
	if err != nil {
		log.Fatalf("failed to create all directory: %v", err)
	}

	// Generate go.mod
	generateAllGoMod(allDirectory, pseudoVersion, builtTargets)

	// Generate platform-specific files
	for _, targetName := range builtTargets {
		generatePlatformImportFile(allDirectory, targetName)
	}

	log.Printf("Generated all package with %d platforms", len(builtTargets))
}

// generateAllGoMod generates the go.mod file for the all package
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

// generatePlatformImportFile generates a platform-specific import file
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

// getBuildTagForTarget returns the build tag for a given target directory name
func getBuildTagForTarget(targetName string) string {
	// Parse target name: {goos}_{goarch} or {goos}_{goarch}_musl
	parts := strings.Split(targetName, "_")
	if len(parts) < 2 {
		log.Fatalf("invalid target name: %s", targetName)
	}

	goos := parts[0]
	goarch := parts[1]
	isMusl := len(parts) >= 3 && parts[2] == "musl"

	// Handle special arch names
	switch goarch {
	case "amd64", "arm64", "arm":
		// Keep as is
	case "386":
		// Keep as is
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

	return fmt.Sprintf("%s && %s", goos, goarch)
}

// runGoModTidy runs go mod tidy with GOPROXY=direct
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
