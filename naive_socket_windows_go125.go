//go:build windows && go1.25

package cronet

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"strconv"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

func createSocketPair() (cronetFD int, proxyConn net.Conn, err error) {
	cronetFD, proxyConn, err = createUnixSocketPair()
	if err == nil {
		return
	}
	return createTCPLoopbackSocketPair()
}

// createUnixSocketPair accepts the proxy end through net.Listen instead of the
// Winsock accept function, so that the accepted socket is owned by the net package
// and released with closesocket. os.NewFile marks a handle as a file, and closing
// such a file calls CloseHandle, which frees the handle value without releasing the
// Winsock state behind it; getsockopt(SO_PROTOCOL_INFO) on a socket that later
// receives the same handle value then reports the released AF_UNIX SOCK_STREAM
// socket, and Cronet applies its length-prefixed framing to a plain UDP socket.
//
// Windows has no abstract socket namespace, so the listener is bound to a file
// under the temporary directory.
func createUnixSocketPair() (cronetFD int, proxyConn net.Conn, err error) {
	socketSuffix, err := randomHexString(8)
	if err != nil {
		return -1, nil, err
	}
	name := filepath.Join(os.TempDir(), "cronet-go-"+strconv.Itoa(os.Getpid())+"-"+socketSuffix+".sock")
	if len(name) >= windows.UNIX_PATH_MAX {
		return -1, nil, E.New("unix socket path too long: ", name)
	}
	_ = os.Remove(name)

	listener, err := net.Listen("unix", name)
	if err != nil {
		return -1, nil, err
	}
	defer listener.Close()

	clientSocket, err := windows.Socket(windows.AF_UNIX, windows.SOCK_STREAM, 0)
	if err != nil {
		return -1, nil, err
	}

	type acceptResult struct {
		conn net.Conn
		err  error
	}
	acceptedChannel := make(chan acceptResult, 1)
	go func() {
		conn, acceptError := listener.Accept()
		acceptedChannel <- acceptResult{conn: conn, err: acceptError}
	}()

	connectError := windows.Connect(clientSocket, &windows.SockaddrUnix{Name: name})
	if connectError != nil {
		_ = windows.Closesocket(clientSocket)
		_ = listener.Close()
		accepted := <-acceptedChannel
		if accepted.conn != nil {
			_ = accepted.conn.Close()
		}
		return -1, nil, connectError
	}

	accepted := <-acceptedChannel
	if accepted.err != nil {
		_ = windows.Closesocket(clientSocket)
		return -1, nil, accepted.err
	}

	return int(clientSocket), accepted.conn, nil
}

func randomHexString(byteCount int) (string, error) {
	randomBytes := make([]byte, byteCount)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(randomBytes), nil
}
