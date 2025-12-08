package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var commandSync = &cobra.Command{
	Use:   "sync",
	Short: "Download Chromium cronet components",
	Run: func(cmd *cobra.Command, args []string) {
		sync()
	},
}

func init() {
	mainCommand.AddCommand(commandSync)
}

func sync() {
	log.Print("Syncing Chromium cronet components...")

	// Read CHROMIUM_VERSION
	versionFile := filepath.Join(naiveRoot, "CHROMIUM_VERSION")
	versionData, err := os.ReadFile(versionFile)
	if err != nil {
		log.Fatalf("failed to read CHROMIUM_VERSION: %v", err)
	}
	version := strings.TrimSpace(string(versionData))
	log.Printf("Chromium version: %s", version)

	// Check if components exist and are committed
	cronetDirectory := filepath.Join(srcRoot, "components", "cronet")
	if _, err := os.Stat(cronetDirectory); err == nil {
		// Directory exists, check if it's committed
		status := runCommandOutput(naiveRoot, "git", "status", "--porcelain", "src/components/cronet")
		if strings.TrimSpace(status) == "" {
			log.Print("Components already up to date")
			return
		}
	}

	// Components to download
	components := []string{"cronet", "grpc_support", "prefs"}

	for _, name := range components {
		log.Printf("Downloading %s...", name)

		url := fmt.Sprintf(
			"https://chromium.googlesource.com/chromium/src/+archive/refs/tags/%s/components/%s.tar.gz",
			version, name)

		destinationDirectory := filepath.Join(srcRoot, "components", name)

		// Remove existing directory
		os.RemoveAll(destinationDirectory)
		err := os.MkdirAll(destinationDirectory, 0o755)
		if err != nil {
			log.Fatalf("failed to create directory %s: %v", destinationDirectory, err)
		}

		// Download and extract
		err = downloadAndExtract(url, destinationDirectory)
		if err != nil {
			log.Fatalf("failed to download %s: %v", name, err)
		}

		log.Printf("Downloaded %s", name)
	}

	// Git add and commit
	log.Print("Creating git commit...")
	runCommand(naiveRoot, "git", "add",
		"src/components/cronet",
		"src/components/grpc_support",
		"src/components/prefs")

	commitMessage := fmt.Sprintf(`Add Chromium cronet components (v%s)

Downloaded from Chromium source:
- components/cronet/
- components/grpc_support/
- components/prefs/

Use 'go run ./cmd/build-naive sync' to re-download.`, version)

	runCommand(naiveRoot, "git", "commit", "-m", commitMessage)

	log.Print("Sync complete!")
}

func downloadAndExtract(url, destinationDirectory string) error {
	response, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, response.Status)
	}

	// Use tar command to extract (simpler than using archive/tar with gzip)
	command := exec.Command("tar", "-xzf", "-", "-C", destinationDirectory)
	command.Stdin = response.Body
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	err = command.Run()
	if err != nil {
		return fmt.Errorf("tar extraction failed: %w", err)
	}

	return nil
}
