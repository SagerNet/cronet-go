//go:build windows

package cronet

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

var winsockSystemLibrary = windows.NewLazySystemDLL("ws2_32.dll")

var (
	winsockProcAccept = winsockSystemLibrary.NewProc("accept")
	winsockProcRecv   = winsockSystemLibrary.NewProc("recv")
	winsockProcSend   = winsockSystemLibrary.NewProc("send")
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
			_ = windows.Closesocket(socket)
		}
		return -1, E.Cause(err, "control raw conn")
	}
	if controlError != nil {
		return -1, controlError
	}
	return int(socket), nil
}

type socketTimeoutError struct{}

func (socketTimeoutError) Error() string   { return "i/o timeout" }
func (socketTimeoutError) Timeout() bool   { return true }
func (socketTimeoutError) Temporary() bool { return true }

type unixSocketAddress struct {
	name string
}

func (a unixSocketAddress) Network() string { return "unix" }
func (a unixSocketAddress) String() string  { return a.name }

type winsockStreamConn struct {
	socketHandle windows.Handle
	closeOnce    sync.Once
}

func newWinsockStreamConn(socketHandle windows.Handle) *winsockStreamConn {
	return &winsockStreamConn{socketHandle: socketHandle}
}

func (c *winsockStreamConn) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	r1, _, err := winsockProcRecv.Call(
		uintptr(c.socketHandle),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
		0,
	)
	n := int(r1)
	if n != -1 {
		return n, nil
	}
	if isWinsockTimeout(err) {
		return 0, socketTimeoutError{}
	}
	return 0, err
}

func (c *winsockStreamConn) Write(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	r1, _, err := winsockProcSend.Call(
		uintptr(c.socketHandle),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
		0,
	)
	n := int(r1)
	if n != -1 {
		return n, nil
	}
	if isWinsockTimeout(err) {
		return 0, socketTimeoutError{}
	}
	return 0, err
}

func (c *winsockStreamConn) Close() error {
	var closeError error
	c.closeOnce.Do(func() {
		closeError = windows.Closesocket(c.socketHandle)
	})
	return closeError
}

func (c *winsockStreamConn) LocalAddr() net.Addr {
	return unixSocketAddress{name: "winsock-unix-local"}
}

func (c *winsockStreamConn) RemoteAddr() net.Addr {
	return unixSocketAddress{name: "winsock-unix-remote"}
}

func (c *winsockStreamConn) SetDeadline(deadline time.Time) error {
	readError := c.SetReadDeadline(deadline)
	writeError := c.SetWriteDeadline(deadline)
	if readError != nil {
		return readError
	}
	return writeError
}

func (c *winsockStreamConn) SetReadDeadline(deadline time.Time) error {
	return setSocketTimeout(c.socketHandle, windows.SO_RCVTIMEO, deadline)
}

func (c *winsockStreamConn) SetWriteDeadline(deadline time.Time) error {
	return setSocketTimeout(c.socketHandle, winsockSO_SNDTIMEO, deadline)
}

func (c *winsockStreamConn) CloseWrite() error {
	return windows.Shutdown(c.socketHandle, windows.SHUT_WR)
}

const winsockSO_SNDTIMEO = 0x1005

func setSocketTimeout(socketHandle windows.Handle, option int, deadline time.Time) error {
	timeoutMilliseconds := 0
	if !deadline.IsZero() {
		timeout := time.Until(deadline)
		if timeout <= 0 {
			timeoutMilliseconds = 1
		} else {
			timeoutMilliseconds = int(timeout / time.Millisecond)
			if timeoutMilliseconds <= 0 {
				timeoutMilliseconds = 1
			}
		}
	}
	return windows.SetsockoptInt(socketHandle, windows.SOL_SOCKET, option, timeoutMilliseconds)
}

func isWinsockTimeout(err error) bool {
	winsockError, ok := err.(syscall.Errno)
	if !ok {
		return false
	}
	return winsockError == windows.WSAETIMEDOUT || winsockError == windows.WSAEWOULDBLOCK
}

func createUnixSocketPair() (cronetSocket windows.Handle, proxySocket windows.Handle, err error) {
	socketSuffix, err := randomHexString(8)
	if err != nil {
		return 0, 0, err
	}
	socketBaseName := "cronet-go-" + strconv.Itoa(os.Getpid()) + "-" + socketSuffix + ".sock"

	candidates := []string{
		"@" + socketBaseName,
	}

	temporaryPathCandidate := filepath.Join(os.TempDir(), socketBaseName)
	if len(temporaryPathCandidate) < windows.UNIX_PATH_MAX {
		candidates = append(candidates, temporaryPathCandidate)
	}

	var lastError error
	for _, name := range candidates {
		cronetSocket, proxySocket, lastError = createUnixSocketPairWithName(name)
		if lastError == nil {
			return cronetSocket, proxySocket, nil
		}
	}
	return 0, 0, lastError
}

func createUnixSocketPairWithName(name string) (cronetSocket windows.Handle, proxySocket windows.Handle, err error) {
	if name != "" && name[0] != '@' {
		_ = os.Remove(name)
	}

	listenerSocket, err := windows.Socket(windows.AF_UNIX, windows.SOCK_STREAM, 0)
	if err != nil {
		return 0, 0, err
	}
	listenerClosed := false
	closeListenerSocket := func() {
		if listenerClosed {
			return
		}
		listenerClosed = true
		_ = windows.Closesocket(listenerSocket)
	}
	defer closeListenerSocket()

	listenerAddress := &windows.SockaddrUnix{Name: name}
	err = windows.Bind(listenerSocket, listenerAddress)
	if err != nil {
		return 0, 0, err
	}
	err = windows.Listen(listenerSocket, 1)
	if err != nil {
		return 0, 0, err
	}

	clientSocket, err := windows.Socket(windows.AF_UNIX, windows.SOCK_STREAM, 0)
	if err != nil {
		return 0, 0, err
	}
	clientClosed := false
	closeClientSocket := func() {
		if clientClosed {
			return
		}
		clientClosed = true
		_ = windows.Closesocket(clientSocket)
	}

	acceptedDone := make(chan struct{})
	var acceptedSocket windows.Handle
	var acceptError error
	go func() {
		defer close(acceptedDone)
		r1, _, callError := winsockProcAccept.Call(uintptr(listenerSocket), 0, 0)
		if uintptr(r1) == uintptr(^uintptr(0)) {
			acceptError = callError
			return
		}
		acceptedSocket = windows.Handle(r1)
	}()

	connectError := windows.Connect(clientSocket, listenerAddress)
	if connectError != nil {
		closeListenerSocket()
		closeClientSocket()
		<-acceptedDone
		if acceptedSocket != 0 {
			_ = windows.Closesocket(acceptedSocket)
		}
		return 0, 0, connectError
	}

	<-acceptedDone
	if acceptError != nil {
		closeClientSocket()
		return 0, 0, acceptError
	}

	closeListenerSocket()
	if name != "" && name[0] != '@' {
		_ = os.Remove(name)
	}

	return acceptedSocket, clientSocket, nil
}

func randomHexString(byteCount int) (string, error) {
	randomBytes := make([]byte, byteCount)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(randomBytes), nil
}

// createSocketPair creates a bidirectional socket pair using TCP loopback.
// This approach is necessary on Windows because Named Pipes are not compatible
// with Cronet's socket layer (ioctlsocket fails on pipe handles).
// Returns the cronet-side socket handle and a net.Conn for the proxy side.
// The caller is responsible for closing both the handle and the connection.
func createSocketPair() (cronetFD int, proxyConn net.Conn, err error) {
	cronetSocket, proxySocket, err := createUnixSocketPair()
	if err == nil {
		return int(cronetSocket), newWinsockStreamConn(proxySocket), nil
	}

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
