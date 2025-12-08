package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

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
	defer os.RemoveAll(temporaryDirectory)

	// Create worktree based on current HEAD (keep all files)
	runCommand(projectRoot, "git", "worktree", "add", temporaryDirectory, "HEAD")

	// Copy lib, include directories and cgo_*.go files
	copyDirectory(filepath.Join(projectRoot, "lib"), filepath.Join(temporaryDirectory, "lib"))
	copyDirectory(filepath.Join(projectRoot, "include"), filepath.Join(temporaryDirectory, "include"))
	copyGlob(projectRoot, temporaryDirectory, "cgo_*.go")

	// Stage and commit (force add to include .gitignore'd files)
	runCommand(temporaryDirectory, "git", "add", "-f", "-A")
	commitMessage := fmt.Sprintf("Build from %s", mainCommit[:8])
	runCommand(temporaryDirectory, "git", "commit", "-m", commitMessage)

	// Force push to target branch
	runCommand(temporaryDirectory, "git", "push", "-f", "origin", "HEAD:"+publishBranch)

	// Cleanup worktree
	runCommand(projectRoot, "git", "worktree", "remove", "--force", temporaryDirectory)

	log.Printf("Published to %s branch!", publishBranch)
}
