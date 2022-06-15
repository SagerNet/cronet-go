package main

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/sagernet/sing-tools/extensions/log"
	"github.com/sagernet/sing/common"
)

var logger = log.NewLogger("build")

func appendEnv(key string, value string) {
	common.Must(os.Setenv(key, strings.TrimSpace(os.ExpandEnv("$"+key+" "+value))))
}

func main() {
	var args []string
	args = append(args, "build")
	args = append(args, "-gcflags", "-c "+strconv.Itoa(runtime.NumCPU()))

	goos := os.Getenv("GOOS")
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := os.Getenv("GOARCH")
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	openwrt := os.Getenv("OPENWRT")

	var disablePie bool
	if goarch == "mipsle" || goarch == "mips64le" {
		disablePie = true
	} else if goos == "windows" && goarch == "arm64" {
		disablePie = true
	} else if goarch == "186" && goos != "android" {
		disablePie = true
	}

	if !disablePie {
		args = append(args, "-buildmode", "pie")
		appendEnv("CGO_LDFLAGS", "-pie")
	} else {
		appendEnv("CGO_LDFLAGS", "-nopie")
	}

	switch goos {
	case "windows":
		os.Setenv("MSYS", "winsymlinks:nativestrict")
	case "linux":
		appendEnv("CGO_CFLAGS", "-I $PWD --sysroot=$PWD/sysroot")
		appendEnv("CGO_LDFLAGS", "-I $PWD --sysroot=$PWD/sysroot")
		appendEnv("CGO_LDFLAGS", "-fuse-ld=lld")
		if openwrt != "" {
			appendEnv("CGO_CFLAGS", "-Wno-error=unused-command-line-argument")

			if strings.HasPrefix(openwrt, "aarch64") {
				appendEnv("CGO_CFLAGS", "--target=aarch64-openwrt-linux-musl -mbranch-protection=pac-ret")
				appendEnv("CGO_LDFLAGS", "--target=aarch64-openwrt-linux-musl")
				switch openwrt {
				case "aarch64_cortex-a53":
					appendEnv("CGO_CFLAGS", "-mcpu=cortex-a53")
				case "aarch64_cortex-a72":
					appendEnv("CGO_CFLAGS", "-mcpu=cortex-a72")
					// case "aarch64_generic":
				}
			} else if strings.HasPrefix(openwrt, "arm") {
				appendEnv("CGO_CFLAGS", "--target=arm-openwrt-linux-muslgnueabi")
				appendEnv("CGO_LDFLAGS", "--target=arm-openwrt-linux-muslgnueabi")

				var armIsSoft bool

				switch openwrt {
				case "arm_cortex-a5_vfpv4":
					appendEnv("CGO_CFLAGS", "-mcpu=cortex-a5 -mfpu=vfpv4")
				case "arm_cortex-a7":
					appendEnv("CGO_CFLAGS", "-mcpu=cortex-a7")
					armIsSoft = true
				case "arm_cortex-a7_neon-vfpv4":
					appendEnv("CGO_CFLAGS", "-mcpu=cortex-a7 -mfpu=neon-vfpv4")
				case "arm_cortex-a8_vfpv3":
					appendEnv("CGO_CFLAGS", "-mcpu=cortex-a8 -mfpu=neon-vfpv3")
				case "arm_cortex-a9":
					appendEnv("CGO_CFLAGS", "-mcpu=cortex-a9")
					armIsSoft = true
				case "arm_cortex-a9_neon":
					appendEnv("CGO_CFLAGS", "-mcpu=cortex-a9 -mfpu=neon")
				case "arm_cortex-a9_vfpv3-d16":
					appendEnv("CGO_CFLAGS", "-mcpu=cortex-a9 -mfpu=vfpv3-d16")
				case "arm_cortex-a15_neon-vfpv4":
					appendEnv("CGO_CFLAGS", "-mcpu=cortex-a15 -mfpu=neon-vfpv4")
				case "arm_arm1176jzf-s_vfp":
					appendEnv("CGO_CFLAGS", "-mcpu=arm1176jzf-s -mfpu=vfp")
				case "arm_arm926ej-s":
					appendEnv("CGO_CFLAGS", "-mcpu=arm926ej-s -marm")
					armIsSoft = true
				}

				if armIsSoft {
					appendEnv("CGO_CFLAGS", "-mfloat-abi=soft")
					appendEnv("CGO_LDFLAGS", "-mfloat-abi=soft")
				} else {
					appendEnv("CGO_CFLAGS", "-mfloat-abi=hard")
					appendEnv("CGO_LDFLAGS", "-mfloat-abi=hard")
				}
			} else {
				switch openwrt {
				case "x86_64":
					appendEnv("CGO_CFLAGS", "--target=x86_64-openwrt-linux-musl -m64 -march=x86-64 -msse3")
					appendEnv("CGO_LDFLAGS", "--target=x86_64-openwrt-linux-musl -m64")
				case "x86":
					appendEnv("CGO_CFLAGS", "--target=i486-openwrt-linux-musl -m32 -mfpmath=sse -msse3")
					appendEnv("CGO_LDFLAGS", "-Wl,--dynamic-linker=/lib/ld-musl-i386.so.1 --target=i486-openwrt-linux-musl -m32")
				case "mipsel_mips32":
					appendEnv("CGO_CFLAGS", "--target=mipsel-openwrt-linux-musl -march=mipsel -mcpu=mips32 -msoft-float")
					appendEnv("CGO_LDFLAGS", "-Wl,--dynamic-linker=/lib/ld-musl-mipsel-sf.so.1 --target=mipsel-openwrt-linux-musl -mips32")
					os.Setenv("GOMIPS", "softfloat")
				case "mipsel_24kc":
					appendEnv("CGO_CFLAGS", "--target=mipsel-openwrt-linux-musl -march=mipsel -mcpu=mips32r2 -msoft-float -mtune=24kc")
					appendEnv("CGO_LDFLAGS", "-Wl,--dynamic-linker=/lib/ld-musl-mipsel-sf.so.1 --target=mipsel-openwrt-linux-musl -mips32r2")
					os.Setenv("GOMIPS", "softfloat")
				case "mipsel_74kc":
					appendEnv("CGO_CFLAGS", "--target=mipsel-openwrt-linux-musl -march=mipsel -mcpu=mips32r2 -msoft-float -mtune=74kc")
					appendEnv("CGO_LDFLAGS", "-Wl,--dynamic-linker=/lib/ld-musl-mipsel-sf.so.1 --target=mipsel-openwrt-linux-musl -mips32r2")
					os.Setenv("GOMIPS", "softfloat")
				}
			}
		} else {
			switch goarch {
			case "amd64":
				appendEnv("CGO_CFLAGS", "-m64 -march=x86-64 -msse3")
				appendEnv("CGO_LDFLAGS", "-m64")
			case "386":
				appendEnv("CGO_CFLAGS", "-m32 -mfpmath=sse -msse3")
				appendEnv("CGO_LDFLAGS", "-m32")
			case "arm64":
				appendEnv("CGO_CFLAGS", "--target=aarch64-linux-gnu -mbranch-protection=pac-ret")
				appendEnv("CGO_LDFLAGS", "--target=aarch64-linux-gnu")
			case "arm":
				appendEnv("CGO_CFLAGS", "--target=arm-linux-gnueabihf -march=armv7-a -mtune=generic-armv7-a -mfpu=neon")
				appendEnv("CGO_LDFLAGS", "--target=arm-linux-gnueabihf -march=armv7-a ")
				os.Setenv("GOARM", "7")
			case "mipsle":
				appendEnv("CGO_CFLAGS", "--target=mipsel-linux-gnu -march=mipsel -mcpu=mips32 -mhard-float")
				appendEnv("CGO_LDFLAGS", "--target=mipsel-linux-gnu -mips32")
			case "mips64le":
				appendEnv("CGO_CFLAGS", "--target=mips64el-linux-gnuabi64 -march=mips64el -mcpu=mips64r2")
				appendEnv("CGO_LDFLAGS", "--target=mips64el-linux-gnuabi64 -mips64r2")
			}
		}
	}

	os.Setenv("PATH", os.ExpandEnv("$PWD/llvm/bin:$PATH"))
	os.Setenv("CC", "clang")
	os.Setenv("CGO_ENABLED", "1")
	os.Setenv("CGO_LDFLAGS_ALLOW", ".*")

	for _, env := range os.Environ() {
		println(env)
	}

	args = append(args, os.Args[1:]...)

	err := execve("go", args...)
	if err != nil {
		logger.Fatal(err)
	}
}

func execve(name string, args ...string) error {
	path, _ := exec.LookPath(name)
	args = append([]string{path}, args...)
	return syscall.Exec(path, args, os.Environ())
}
