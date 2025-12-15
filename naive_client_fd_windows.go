//go:build windows

package cronet

import (
	"net"
	"syscall"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

// dupSocketFD extracts and duplicates the socket handle from a syscall.Conn.
// Returns a new independent handle; the caller should close both the returned handle (after use)
// and the original connection (immediately after this call).
func dupSocketFD(syscallConn syscall.Conn) (int, error) {
	rawConn, err := syscallConn.SyscallConn()
	if err != nil {
		return -1, E.Cause(err, "get syscall conn")
	}
	var socket windows.Handle
	var controlError error
	err = rawConn.Control(func(fdPtr uintptr) {
		// Duplicate the socket handle so we can transfer ownership
		currentProcess := windows.CurrentProcess()
		duplicateError := windows.DuplicateHandle(
			currentProcess,
			windows.Handle(fdPtr),
			currentProcess,
			&socket,
			0,
			false,
			windows.DUPLICATE_SAME_ACCESS,
		)
		if duplicateError != nil {
			controlError = E.Cause(duplicateError, "duplicate socket handle")
			return
		}
	})
	if err != nil {
		// Close duplicated handle if Control itself fails after DuplicateHandle succeeded
		if socket != 0 {
			windows.CloseHandle(socket)
		}
		return -1, E.Cause(err, "control raw conn")
	}
	if controlError != nil {
		return -1, controlError
	}
	return int(socket), nil
}

// createSocketPair creates a bidirectional socket pair using TCP loopback.
// This approach is necessary on Windows because Named Pipes are not compatible
// with Cronet's socket layer (ioctlsocket fails on pipe handles).
// Returns the cronet-side socket handle and a net.Conn for the proxy side.
// The caller is responsible for closing both the handle and the connection.
func createSocketPair() (cronetFD int, proxyConn net.Conn, err error) {
	// Listen on random port on loopback interface
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return -1, nil, E.Cause(err, "create loopback listener")
	}
	defer listener.Close()

	// Connect client in parallel
	var clientConn net.Conn
	var clientError error
	done := make(chan struct{})
	go func() {
		clientConn, clientError = net.Dial("tcp", listener.Addr().String())
		close(done)
	}()

	// Accept the connection
	serverConn, err := listener.Accept()
	if err != nil {
		<-done
		if clientConn != nil {
			clientConn.Close()
		}
		return -1, nil, E.Cause(err, "accept loopback connection")
	}

	<-done
	if clientError != nil {
		serverConn.Close()
		return -1, nil, E.Cause(clientError, "dial loopback")
	}

	// Extract and duplicate socket handle from server side
	tcpConn := serverConn.(*net.TCPConn)
	fd, err := dupSocketFD(tcpConn)
	if err != nil {
		serverConn.Close()
		clientConn.Close()
		return -1, nil, E.Cause(err, "dup loopback socket")
	}

	// Close original connection; the duplicated handle is independent
	serverConn.Close()
	return fd, clientConn, nil
}
