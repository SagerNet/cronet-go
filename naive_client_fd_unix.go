//go:build unix

package cronet

import (
	"net"
	"os"
	"syscall"

	E "github.com/sagernet/sing/common/exceptions"
)

// dupSocketFD extracts and duplicates the socket file descriptor from a syscall.Conn.
// Returns a new independent FD; the caller should close both the returned FD (after use)
// and the original connection (immediately after this call).
func dupSocketFD(syscallConn syscall.Conn) (int, error) {
	rawConn, err := syscallConn.SyscallConn()
	if err != nil {
		return -1, E.Cause(err, "get syscall conn")
	}
	var fd int
	var controlError error
	err = rawConn.Control(func(fdPtr uintptr) {
		// Duplicate the file descriptor so we can transfer ownership
		newFD, dupError := syscall.Dup(int(fdPtr))
		if dupError != nil {
			controlError = E.Cause(dupError, "dup socket fd")
			return
		}
		syscall.CloseOnExec(newFD)
		fd = newFD
	})
	if err != nil {
		return -1, E.Cause(err, "control raw conn")
	}
	if controlError != nil {
		return -1, controlError
	}
	return fd, nil
}

// createSocketPair creates a bidirectional socket pair for the proxy fallback.
// Returns the cronet-side FD and a net.Conn for the proxy side.
// The caller is responsible for closing both the FD and the connection.
func createSocketPair() (cronetFD int, proxyConn net.Conn, err error) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return -1, nil, E.Cause(err, "create socketpair")
	}

	syscall.CloseOnExec(fds[0])

	// fds[0] goes to cronet, fds[1] wraps as net.Conn for proxy.
	// FileConn duplicates the fd, so close the original.
	file := os.NewFile(uintptr(fds[1]), "cronet-socketpair")
	conn, err := net.FileConn(file)
	_ = file.Close()
	if err != nil {
		syscall.Close(fds[0])
		return -1, nil, E.Cause(err, "create net conn from socketpair")
	}
	return fds[0], conn, nil
}
