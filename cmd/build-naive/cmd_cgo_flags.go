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

	linkFlags, err := extractLinkFlags(outputDirectory)
	if err != nil {
		log.Fatalf("failed to extract link flags: %v", err)
	}

	var ldFlags []string

	libraryDirectory := filepath.Join(projectRoot, "lib", getLibraryDirectoryName(t))
	if t.GOOS == "darwin" || t.GOOS == "ios" {
		ldFlags = append(ldFlags, filepath.Join(libraryDirectory, "libcronet.a"))
	} else {
		ldFlags = append(ldFlags, "-L"+libraryDirectory, "-l:libcronet.a")
	}

	ldFlags = append(ldFlags, linkFlags.LDFlags...)
	ldFlags = append(ldFlags, linkFlags.Libs...)
	ldFlags = append(ldFlags, linkFlags.Frameworks...)

	if t.GOOS == "linux" {
		ldFlags = append(ldFlags, "-fuse-ld=lld")
		// Add -no-pie for 32-bit (required for position-dependent code)
		if t.ARCH == "386" || t.ARCH == "arm" {
			ldFlags = append(ldFlags, "-no-pie")
		}
		if t.Libc == "musl" {
			ldFlags = append(ldFlags, "-static")
		}
	}

	fmt.Println(strings.Join(ldFlags, " "))
}
