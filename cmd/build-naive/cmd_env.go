package main

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	commandEnv = &cobra.Command{
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
	envExport bool
)

func init() {
	mainCommand.AddCommand(commandEnv)
	commandEnv.Flags().BoolVar(&envExport, "export", false, "Prefix output with 'export' for use with eval")
}

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
		case "loong64":
			return "loongarch64-openwrt-linux-musl"
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
	case "loong64":
		return "loongarch64-linux-gnu"
	}
	return ""
}

func getSysrootPath(t Target) string {
	if t.Libc == "musl" {
		config := getOpenwrtConfig(t)
		return filepath.Join(srcRoot, "out/sysroot-build/openwrt", config.release, config.arch)
	}
	sysrootInfo := map[string]struct{ arch, release string }{
		"x64":     {"amd64", "bullseye"},
		"arm64":   {"arm64", "bullseye"},
		"x86":     {"i386", "bullseye"},
		"arm":     {"armhf", "bullseye"},
		"loong64": {"loong64", "sid"},
	}[t.CPU]
	return filepath.Join(srcRoot, "out/sysroot-build", sysrootInfo.release, sysrootInfo.release+"_"+sysrootInfo.arch+"_staging")
}

func printEnv(t Target) {
	if t.GOOS == "windows" {
		log.Fatal("env command is not supported for Windows (use purego mode with embedded DLL)")
	}

	prefix := ""
	if envExport {
		prefix = "export "
	}

	// CGO_LDFLAGS: Only output toolchain flags that cannot be in #cgo LDFLAGS.
	// Library paths and system libs are in the generated lib_*_cgo.go files.
	if t.GOOS == "linux" {
		var ldFlags []string
		ldFlags = append(ldFlags, "-fuse-ld=lld")
		if t.ARCH == "386" || t.ARCH == "arm" || t.ARCH == "loong64" {
			ldFlags = append(ldFlags, "-Wl,-no-pie")
		}
		fmt.Printf("%sCGO_LDFLAGS=%s\n", prefix, shellQuote(strings.Join(ldFlags, " "), envExport))
	}
	// Darwin/iOS: No CGO_LDFLAGS needed, all flags are in the generated cgo files

	// Linux-specific: CC, CXX for cross-compilation, QEMU_LD_PREFIX for running binaries
	if t.GOOS == "linux" {
		clangPath := filepath.Join(srcRoot, "third_party/llvm-build/Release+Asserts/bin/clang")
		clangTarget := getClangTarget(t)
		sysroot := getSysrootPath(t)

		cc := fmt.Sprintf("%s --target=%s --sysroot=%s", clangPath, clangTarget, sysroot)
		cxx := fmt.Sprintf("%s++ --target=%s --sysroot=%s", clangPath, clangTarget, sysroot)

		fmt.Printf("%sCC=%s\n", prefix, shellQuote(cc, envExport))
		fmt.Printf("%sCXX=%s\n", prefix, shellQuote(cxx, envExport))
		fmt.Printf("%sQEMU_LD_PREFIX=%s\n", prefix, sysroot)
	}
}

func shellQuote(s string, quote bool) string {
	if quote && strings.ContainsAny(s, " \t\n\"'\\$") {
		return "\"" + strings.ReplaceAll(s, "\"", "\\\"") + "\""
	}
	return s
}
