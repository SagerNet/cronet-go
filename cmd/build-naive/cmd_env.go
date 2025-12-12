package main

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/spf13/cobra"
)

var commandEnv = &cobra.Command{
	Use:   "env",
	Short: "Output environment variables for building downstream projects",
	Run: func(cmd *cobra.Command, args []string) {
		targets := parseTargets()
		if len(targets) != 1 {
			log.Fatal("env requires exactly one target")
		}
		printEnv(targets[0])
	},
}

func init() {
	mainCommand.AddCommand(commandEnv)
}

// getClangTarget returns the clang target triple for a target
func getClangTarget(t Target) string {
	if t.Libc == "musl" {
		switch t.CPU {
		case "x64":
			return "x86_64-openwrt-linux-musl"
		case "arm64":
			return "aarch64-openwrt-linux-musl"
		case "x86":
			return "i486-openwrt-linux-musl"
		case "arm":
			return "arm-openwrt-linux-musleabi"
		}
	}
	switch t.CPU {
	case "x64":
		return "x86_64-linux-gnu"
	case "arm64":
		return "aarch64-linux-gnu"
	case "x86":
		return "i686-linux-gnu"
	case "arm":
		return "arm-linux-gnueabihf"
	}
	return ""
}

// getSysrootPath returns the sysroot path for a target
func getSysrootPath(t Target) string {
	if t.Libc == "musl" {
		config := getOpenwrtConfig(t)
		return filepath.Join(srcRoot, "out/sysroot-build/openwrt", config.release, config.arch)
	}
	sysrootArch := map[string]string{
		"x64":   "amd64",
		"arm64": "arm64",
		"x86":   "i386",
		"arm":   "armhf",
	}[t.CPU]
	return filepath.Join(srcRoot, "out/sysroot-build/bullseye", "bullseye_"+sysrootArch+"_staging")
}

func printEnv(t Target) {
	if t.GOOS != "linux" {
		log.Fatal("env command is only supported for Linux targets")
	}

	clangPath := filepath.Join(srcRoot, "third_party/llvm-build/Release+Asserts/bin/clang")
	clangTarget := getClangTarget(t)
	sysroot := getSysrootPath(t)

	cc := fmt.Sprintf("%s --target=%s --sysroot=%s", clangPath, clangTarget, sysroot)
	cxx := fmt.Sprintf("%s++ --target=%s --sysroot=%s", clangPath, clangTarget, sysroot)

	fmt.Printf("CC=%q\n", cc)
	fmt.Printf("CXX=%q\n", cxx)
}
