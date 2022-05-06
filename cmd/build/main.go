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
	var args []string
	args = append(args, "build")
	args = append(args, "-v")
	args = append(args, "-gcflags", "-c "+strconv.Itoa(runtime.NumCPU()))
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
