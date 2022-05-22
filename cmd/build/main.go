package main

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/log"
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
		appendEnv("MSYS", "winsymlinks:nativestrict")
	case "linux":
		appendEnv("CGO_CFLAGS", "-I $PWD --sysroot=$PWD/sysroot")
		appendEnv("CGO_LDFLAGS", "-I $PWD --sysroot=$PWD/sysroot")
		appendEnv("CGO_LDFLAGS", "-fuse-ld=lld")
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
		}

	}

	os.Setenv("PATH", os.ExpandEnv("$PWD/llvm/bin:$PATH"))
	os.Setenv("CC", "clang")
	os.Setenv("CGO_ENABLED", "1")
	os.Setenv("CGO_LDFLAGS_ALLOW", ".*")

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
