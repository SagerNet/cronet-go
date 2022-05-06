package main

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"syscall"

	"github.com/sagernet/sing/common/log"
)

var logger = log.NewLogger("build")

func main() {
	os.Setenv("CGO_LDFLAGS_ALLOW", "-fuse-ld=lld")
	os.Setenv("CC", "clang")

	var args []string
	args = append(args, "build")
	args = append(args, "-x")
	args = append(args, "-o", "naive-go")
	args = append(args, "-gcflags", "-c "+strconv.Itoa(runtime.NumCPU()))
	args = append(args, "-tags", "static")
	args = append(args, os.Args[1:]...)
	args = append(args, "./naive")

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
