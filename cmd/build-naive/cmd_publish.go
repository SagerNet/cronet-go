package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var commandPublish = &cobra.Command{
	Use:   "publish",
	Short: "Commit to go branch and push",
	Run: func(cmd *cobra.Command, args []string) {
		publish()
	},
}

func init() {
	mainCommand.AddCommand(commandPublish)
}

func publish() {
	log.Print("Publishing to go branch...")

	// Get current commit
	mainCommit := strings.TrimSpace(runCommandOutput(projectRoot, "git", "rev-parse", "HEAD"))

	// Create temp directory for worktree
	temporaryDirectory, err := os.MkdirTemp("", "cronet-go-publish-")
	if err != nil {
		log.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(temporaryDirectory)

	// Create worktree based on current HEAD
	runCommand(projectRoot, "git", "worktree", "add", temporaryDirectory, "HEAD")
	runCommand(temporaryDirectory, "git", "rm", "-rf", ".")

	// Files to copy
	filesToCopy := []string{
		"*.go",
		"go.mod",
		"go.sum",
		"include",
		"lib",
		"naive",
		"LICENSE",
		"README.md",
	}

	// Copy files
	for _, pattern := range filesToCopy {
		copyGlob(projectRoot, temporaryDirectory, pattern)
	}

	// Stage and commit (force add to include .gitignore'd files)
	runCommand(temporaryDirectory, "git", "add", "-f", "-A")
	commitMessage := fmt.Sprintf("Build from %s", mainCommit[:8])
	runCommand(temporaryDirectory, "git", "commit", "-m", commitMessage)

	// Force push to go branch
	runCommand(temporaryDirectory, "git", "push", "-f", "origin", "HEAD:go")

	// Cleanup worktree
	runCommand(projectRoot, "git", "worktree", "remove", "--force", temporaryDirectory)

	log.Print("Published to go branch!")
}
