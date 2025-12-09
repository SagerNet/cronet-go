package main

import (
	"log"

	"github.com/spf13/cobra"
)

var commandDownloadToolchain = &cobra.Command{
	Use:   "download-toolchain",
	Short: "Download clang and sysroot without building",
	Run: func(cmd *cobra.Command, args []string) {
		targets := parseTargets()
		downloadToolchain(targets)
	},
}

func init() {
	mainCommand.AddCommand(commandDownloadToolchain)
}

func downloadToolchain(targets []Target) {
	log.Printf("Downloading toolchain for %d target(s)", len(targets))

	for _, t := range targets {
		if t.Libc == "musl" {
			log.Printf("Downloading toolchain for %s/%s (musl)...", t.GOOS, t.ARCH)
		} else {
			log.Printf("Downloading toolchain for %s/%s...", t.GOOS, t.ARCH)
		}
		runGetClang(t)
	}

	log.Print("Toolchain download complete!")
}
