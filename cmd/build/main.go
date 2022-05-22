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

	switch goos {
	case "windows":
		appendEnv("MSYS", "winsymlinks:nativestrict")
	case "linux":
		appendEnv("CGO_CFLAGS", "-I $PWD --sysroot=$PWD/sysroot")
		appendEnv("CGO_LDFLAGS", "-I $PWD --sysroot=$PWD/sysroot")
		appendEnv("CGO_LDFLAGS", "-fuse-ld=lld")

		switch goarch {
		case "amd64":
			args = append(args, "-buildmode", "pie")
		case "arm64":
			appendEnv("CGO_CFLAGS", "--target=aarch64-linux-gnu -mbranch-protection=pac-ret")
			appendEnv("CGO_LDFLAGS", "--target=aarch64-linux-gnu")
			args = append(args, "-buildmode", "pie")
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
