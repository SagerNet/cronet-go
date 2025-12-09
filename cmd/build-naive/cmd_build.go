package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var commandBuild = &cobra.Command{
	Use:   "build",
	Short: "Build cronet_static for specified targets",
	Run: func(cmd *cobra.Command, args []string) {
		targets := parseTargets()
		build(targets)
	},
}

func init() {
	mainCommand.AddCommand(commandBuild)
}

func build(targets []Target) {
	log.Printf("Building cronet_static for %d target(s)", len(targets))

	for _, t := range targets {
		if t.Libc == "musl" {
			log.Printf("Building %s/%s (musl)...", t.GOOS, t.ARCH)
		} else {
			log.Printf("Building %s/%s...", t.GOOS, t.ARCH)
		}
		buildTarget(t)
	}

	log.Print("Build complete!")
}

// getExtraFlags returns the EXTRA_FLAGS for a target
func getExtraFlags(t Target) string {
	flags := []string{
		fmt.Sprintf(`target_os="%s"`, t.OS),
		fmt.Sprintf(`target_cpu="%s"`, t.CPU),
	}
	return strings.Join(flags, " ")
}

// OpenWrt SDK configuration for each architecture
type openwrtConfig struct {
	target    string // OpenWrt target (e.g., "x86", "armsr")
	subtarget string // OpenWrt subtarget (e.g., "64", "generic")
	arch      string // OpenWrt arch (e.g., "x86_64", "aarch64")
	release   string // OpenWrt release version
	gccVer    string // GCC version in SDK
}

// getOpenwrtConfig returns the OpenWrt SDK configuration for a target
func getOpenwrtConfig(t Target) openwrtConfig {
	// Use OpenWrt 23.05.5 as it's stable and widely available
	// GCC version varies by release
	switch t.CPU {
	case "x64":
		return openwrtConfig{
			target:    "x86",
			subtarget: "64",
			arch:      "x86_64",
			release:   "23.05.5",
			gccVer:    "12.3.0",
		}
	case "arm64":
		return openwrtConfig{
			target:    "armsr",
			subtarget: "armv8",
			arch:      "aarch64",
			release:   "23.05.5",
			gccVer:    "12.3.0",
		}
	case "x86":
		return openwrtConfig{
			target:    "x86",
			subtarget: "generic",
			arch:      "i386_pentium4",
			release:   "23.05.5",
			gccVer:    "12.3.0",
		}
	case "arm":
		return openwrtConfig{
			target:    "armsr",
			subtarget: "armv7",
			arch:      "arm_cortex-a15_neon-vfpv4",
			release:   "23.05.5",
			gccVer:    "12.3.0",
		}
	default:
		log.Fatalf("unsupported CPU for musl: %s", t.CPU)
		return openwrtConfig{}
	}
}

// getOpenwrtFlags returns the OPENWRT_FLAGS environment variable value
func getOpenwrtFlags(t Target) string {
	config := getOpenwrtConfig(t)
	return fmt.Sprintf(
		`target="%s" subtarget="%s" arch="%s" release="%s" gcc_ver="%s"`,
		config.target, config.subtarget, config.arch, config.release, config.gccVer,
	)
}

// runGetClang runs naiveproxy's get-clang.sh with appropriate EXTRA_FLAGS
func runGetClang(t Target) {
	// For cross-compilation on Linux, we need to also build host sysroot first
	// because GN needs host sysroot in addition to target sysroot.
	// This applies to linux, android, and openwrt targets.
	hostOS := runtime.GOOS
	hostCPU := hostToCPU(runtime.GOARCH)
	needsHostSysroot := hostOS == "linux" && (t.OS == "linux" || t.OS == "android" || t.OS == "openwrt")
	if needsHostSysroot {
		// Run get-clang.sh with host target to ensure host sysroot is downloaded
		hostFlags := fmt.Sprintf(`target_os="linux" target_cpu="%s"`, hostCPU)
		log.Printf("Running get-clang.sh for host sysroot with EXTRA_FLAGS=%s", hostFlags)
		command := exec.Command("bash", "./get-clang.sh")
		command.Dir = srcRoot
		command.Env = append(os.Environ(), "EXTRA_FLAGS="+hostFlags)
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		err := command.Run()
		if err != nil {
			log.Fatalf("get-clang.sh (host) failed: %v", err)
		}

		// Create symlink for host sysroot so GN can find it at the default location
		hostSysrootSource := filepath.Join(srcRoot, "out/sysroot-build/bullseye/bullseye_amd64_staging")
		hostSysrootDestination := filepath.Join(srcRoot, "build/linux/debian_bullseye_amd64-sysroot")
		if _, err := os.Stat(hostSysrootDestination); os.IsNotExist(err) {
			log.Printf("Creating symlink for host sysroot: %s -> %s", hostSysrootDestination, hostSysrootSource)
			err := os.Symlink(hostSysrootSource, hostSysrootDestination)
			if err != nil {
				log.Fatalf("failed to create host sysroot symlink: %v", err)
			}
		}
	}

	extraFlags := getExtraFlags(t)
	env := []string{"EXTRA_FLAGS=" + extraFlags}

	// For openwrt (musl), also set OPENWRT_FLAGS
	if t.OS == "openwrt" {
		openwrtFlags := getOpenwrtFlags(t)
		env = append(env, "OPENWRT_FLAGS="+openwrtFlags)
		log.Printf("Running get-clang.sh with EXTRA_FLAGS=%s OPENWRT_FLAGS=%s", extraFlags, openwrtFlags)
	} else {
		log.Printf("Running get-clang.sh with EXTRA_FLAGS=%s", extraFlags)
	}

	command := exec.Command("bash", "./get-clang.sh")
	command.Dir = srcRoot
	command.Env = append(os.Environ(), env...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err := command.Run()
	if err != nil {
		log.Fatalf("get-clang.sh failed: %v", err)
	}
}

func buildTarget(t Target) {
	// Run get-clang.sh to ensure toolchain is available
	runGetClang(t)

	outputDirectory := fmt.Sprintf("out/cronet-%s-%s", t.OS, t.CPU)

	// Prepare GN args
	args := []string{
		"is_official_build=true",
		"is_debug=false",
		"is_clang=true",
		"use_thin_lto=false", // Disable ThinLTO so static lib can be linked with system clang
		"fatal_linker_warnings=false",
		"treat_warnings_as_errors=false",
		"is_cronet_build=true",
		"use_udev=false",
		"use_aura=false",
		"use_ozone=false",
		"use_gio=false",
		"use_platform_icu_alternatives=true",
		"use_glib=false",
		"disable_file_support=true",
		"enable_websockets=false",
		"use_kerberos=false",
		"disable_zstd_filter=false",
		"enable_mdns=false",
		"enable_reporting=false",
		"include_transport_security_state_preload_list=false",
		"enable_device_bound_sessions=false",
		"enable_bracketed_proxy_uris=true",
		"enable_quic_proxy_support=true",
		"enable_disk_cache_sql_backend=false",
		"use_nss_certs=false",
		"enable_backup_ref_ptr_support=false",
		"enable_dangling_raw_ptr_checks=false",
		"exclude_unwind_tables=true",
		"enable_resource_allowlist_generation=false",
		"symbol_level=0",
		"enable_dsyms=false",
		"optimize_for_size=true",
		fmt.Sprintf("target_os=\"%s\"", t.OS),
		fmt.Sprintf("target_cpu=\"%s\"", t.CPU),
	}

	// Platform-specific args
	switch t.OS {
	case "mac":
		args = append(args, "use_sysroot=false")
	case "linux":
		// Sysroot is handled by get-clang.sh, use the naiveproxy path
		sysrootArch := map[string]string{
			"x64":   "amd64",
			"arm64": "arm64",
			"x86":   "i386",
			"arm":   "armhf",
		}[t.CPU]
		sysrootDirectory := fmt.Sprintf("out/sysroot-build/bullseye/bullseye_%s_staging", sysrootArch)
		args = append(args, "use_sysroot=true", fmt.Sprintf("target_sysroot=\"//%s\"", sysrootDirectory))
		if t.CPU == "x64" {
			args = append(args, "use_cfi_icall=false", "is_cfi=false")
		}
	case "openwrt":
		// OpenWrt uses musl libc
		config := getOpenwrtConfig(t)
		sysrootDirectory := fmt.Sprintf("out/sysroot-build/openwrt/%s/%s", config.release, config.arch)
		args = append(args,
			"use_sysroot=true",
			fmt.Sprintf("target_sysroot=\"//%s\"", sysrootDirectory),
			"build_static=true", // Static linking for musl
		)
		if t.CPU == "x64" {
			args = append(args, "use_cfi_icall=false", "is_cfi=false")
		}
	case "win":
		args = append(args, "use_sysroot=false")
	case "android":
		args = append(args,
			"use_sysroot=false",
			"default_min_sdk_version=21",
			"is_high_end_android=true",
			"android_ndk_major_version=28",
		)
	case "ios":
		args = append(args,
			"use_sysroot=false",
			"ios_enable_code_signing=false",
			`target_environment="device"`,
			`ios_deployment_target="15.0"`,
		)
	}

	// Detect and use ccache/sccache
	if runtime.GOOS == "windows" {
		sccachePath, _ := exec.LookPath("sccache")
		if sccachePath != "" {
			args = append(args, fmt.Sprintf(`cc_wrapper="%s"`, sccachePath))
		}
	} else {
		ccachePath, _ := exec.LookPath("ccache")
		if ccachePath != "" {
			args = append(args, fmt.Sprintf(`cc_wrapper="%s"`, ccachePath))
		}
	}

	gnArgs := strings.Join(args, " ")

	// Determine GN path
	gnPath := filepath.Join(srcRoot, "gn", "out", "gn")
	if runtime.GOOS == "windows" {
		gnPath += ".exe"
	}

	// Run gn gen
	log.Printf("Running: gn gen %s", outputDirectory)
	gnCommand := exec.Command(gnPath, "gen", outputDirectory, "--args="+gnArgs)
	gnCommand.Dir = srcRoot
	gnCommand.Stdout = os.Stdout
	gnCommand.Stderr = os.Stderr
	// On Windows, use system Visual Studio instead of depot_tools
	if runtime.GOOS == "windows" {
		gnCommand.Env = append(os.Environ(), "DEPOT_TOOLS_WIN_TOOLCHAIN=0")
	}
	err := gnCommand.Run()
	if err != nil {
		log.Fatalf("gn gen failed: %v", err)
	}

	// Run ninja
	log.Printf("Running: ninja -C %s cronet_static", outputDirectory)
	runCommand(srcRoot, "ninja", "-C", outputDirectory, "cronet_static")
}
