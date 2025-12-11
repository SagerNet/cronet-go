package main

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var commandCGOFlags = &cobra.Command{
	Use:   "cgo-flags",
	Short: "Output CGO_LDFLAGS for specified targets",
	Run: func(cmd *cobra.Command, args []string) {
		targets := parseTargets()
		if len(targets) != 1 {
			log.Fatal("cgo-flags requires exactly one target")
		}
		printCGOFlags(targets[0])
	},
}

func init() {
	mainCommand.AddCommand(commandCGOFlags)
}

func printCGOFlags(t Target) {
	outputDirectory := fmt.Sprintf("out/cronet-%s-%s", t.OS, t.CPU)

	// Extract linking flags from ninja file
	linkFlags, err := extractLinkFlags(outputDirectory)
	if err != nil {
		log.Fatalf("failed to extract link flags: %v", err)
	}

	var ldFlags []string

	// Add static library reference
	libraryDirectory := filepath.Join(projectRoot, "lib", getLibraryDirectoryName(t))
	if t.GOOS == "darwin" || t.GOOS == "ios" {
		ldFlags = append(ldFlags, filepath.Join(libraryDirectory, "libcronet.a"))
	} else {
		ldFlags = append(ldFlags, "-L"+libraryDirectory, "-l:libcronet.a")
	}

	// Add extracted ldflags (e.g., -Wl,-wrap,* for Android)
	ldFlags = append(ldFlags, linkFlags.LDFlags...)

	// Add extracted libs
	ldFlags = append(ldFlags, linkFlags.Libs...)

	// Add extracted frameworks
	ldFlags = append(ldFlags, linkFlags.Frameworks...)

	fmt.Println(strings.Join(ldFlags, " "))
}
