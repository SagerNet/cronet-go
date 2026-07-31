//go:build windows

package cronet

import (
	"net"
	"os"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

type fileConn interface {
	File() (*os.File, error)
}

func dupSocketFD(conn fileConn) (int, error) {
	// net.Conn.File first disassociates the source socket from Go's IOCP,
	// then duplicates it with WSADuplicateSocket. This must happen before
	// Cronet starts its own independent overlapped I/O on the socket.
	file, err := conn.File()
	if err != nil {
		return -1, E.Cause(err, "prepare socket for transfer")
	}
	defer file.Close()

	socket := windows.InvalidHandle
	var protocolInfo windows.WSAProtocolInfo
	err = windows.WSADuplicateSocket(
		windows.Handle(file.Fd()),
		uint32(os.Getpid()),
		&protocolInfo,
	)
	if err != nil {
		return -1, E.Cause(err, "duplicate Winsock protocol info")
	}
	socket, err = windows.WSASocket(
		-1,
		-1,
		-1,
		&protocolInfo,
		0,
		windows.WSA_FLAG_OVERLAPPED|windows.WSA_FLAG_NO_HANDLE_INHERIT,
	)
	if err != nil {
		if socket != windows.InvalidHandle {
			_ = windows.Closesocket(socket)
		}
		return -1, E.Cause(err, "create duplicated Winsock socket")
	}
	return int(socket), nil
}

func createTCPLoopbackSocketPair() (cronetFD int, proxyConn net.Conn, err error) {
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		return -1, nil, E.Cause(err, "create loopback listener")
	}
	defer listener.Close()

	socket, err := windows.WSASocket(
		windows.AF_INET,
		windows.SOCK_STREAM,
		windows.IPPROTO_TCP,
		nil,
		0,
		windows.WSA_FLAG_OVERLAPPED|windows.WSA_FLAG_NO_HANDLE_INHERIT,
	)
	if err != nil {
		return -1, nil, E.Cause(err, "create loopback socket")
	}

	listenerAddress := listener.Addr().(*net.TCPAddr)
	sockaddr := &windows.SockaddrInet4{Port: listenerAddress.Port}
	copy(sockaddr.Addr[:], listenerAddress.IP.To4())
	if err = windows.Connect(socket, sockaddr); err != nil {
		_ = windows.Closesocket(socket)
		return -1, nil, E.Cause(err, "connect loopback socket")
	}

	serverConn, err := listener.AcceptTCP()
	if err != nil {
		_ = windows.Closesocket(socket)
		return -1, nil, E.Cause(err, "accept loopback connection")
	}

	return int(socket), serverConn, nil
}
