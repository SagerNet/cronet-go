//go:build windows

package cronet

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

// framedPacketConn wraps a stream socket and provides packet semantics using
// length-prefix framing. Each packet is prefixed with a 2-byte big-endian length.
type framedPacketConn struct {
	conn      net.Conn
	readMutex sync.Mutex
}

var _ net.PacketConn = (*framedPacketConn)(nil)

func newFramedPacketConn(conn net.Conn) *framedPacketConn {
	return &framedPacketConn{conn: conn}
}

func (c *framedPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	c.readMutex.Lock()
	defer c.readMutex.Unlock()

	// Read 2-byte length prefix
	var lengthBuf [2]byte
	_, err = io.ReadFull(c.conn, lengthBuf[:])
	if err != nil {
		return 0, nil, err
	}
	length := binary.BigEndian.Uint16(lengthBuf[:])

	if int(length) > len(p) {
		return 0, nil, errors.New("buffer too small for packet")
	}

	// Read payload
	n, err = io.ReadFull(c.conn, p[:length])
	return n, nil, err
}

func (c *framedPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	if len(p) > 65535 {
		return 0, errors.New("packet too large")
	}

	// Write length prefix + payload atomically
	frame := make([]byte, 2+len(p))
	binary.BigEndian.PutUint16(frame[:2], uint16(len(p)))
	copy(frame[2:], p)

	_, err = c.conn.Write(frame)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *framedPacketConn) Close() error {
	return c.conn.Close()
}

func (c *framedPacketConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *framedPacketConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *framedPacketConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *framedPacketConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

// Read implements net.Conn for connected packet sockets.
// For framedPacketConn, this reads a single framed packet.
func (c *framedPacketConn) Read(p []byte) (n int, err error) {
	n, _, err = c.ReadFrom(p)
	return
}

// Write implements net.Conn for connected packet sockets.
// For framedPacketConn, this writes a single framed packet.
func (c *framedPacketConn) Write(p []byte) (n int, err error) {
	return c.WriteTo(p, nil)
}

// RemoteAddr implements net.Conn. Returns nil for connected socketpair.
func (c *framedPacketConn) RemoteAddr() net.Addr {
	return nil
}

func createPacketSocketPair(forceUDPLoopback bool) (cronetFD int, proxyConn net.PacketConn, err error) {
	if forceUDPLoopback {
		return createUDPLoopbackPair()
	}

	// Create AF_UNIX SOCK_STREAM socket pair
	cronetSocket, proxySocket, err := createUnixSocketPair()
	if err != nil {
		// Fallback to UDP loopback pair
		return createUDPLoopbackPair()
	}

	// Wrap the proxy side with framed packet conn
	return int(cronetSocket), newFramedPacketConn(newWinsockStreamConn(proxySocket)), nil
}

// createUDPLoopbackPair creates a UDP socket pair using loopback as fallback.
func createUDPLoopbackPair() (cronetFD int, proxyConn net.PacketConn, err error) {
	// Create a UDP listener on loopback
	proxyAddress, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		return -1, nil, err
	}
	proxyUDPConn, err := net.ListenUDP("udp", proxyAddress)
	if err != nil {
		return -1, nil, err
	}
	proxyLocalAddress := proxyUDPConn.LocalAddr().(*net.UDPAddr)

	// Create cronet-side UDP socket and connect to proxy
	cronetAddress, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		proxyUDPConn.Close()
		return -1, nil, err
	}
	cronetConn, err := net.DialUDP("udp", cronetAddress, proxyLocalAddress)
	if err != nil {
		proxyUDPConn.Close()
		return -1, nil, err
	}
	cronetLocalAddress := cronetConn.LocalAddr().(*net.UDPAddr)

	// Connect proxyUDPConn to cronetConn's address using syscall
	proxyRawConn, err := proxyUDPConn.SyscallConn()
	if err != nil {
		cronetConn.Close()
		proxyUDPConn.Close()
		return -1, nil, err
	}

	var connectError error
	err = proxyRawConn.Control(func(fd uintptr) {
		sockaddr := &windows.SockaddrInet4{Port: cronetLocalAddress.Port}
		copy(sockaddr.Addr[:], cronetLocalAddress.IP.To4())
		connectError = windows.Connect(windows.Handle(fd), sockaddr)
	})
	if err != nil {
		cronetConn.Close()
		proxyUDPConn.Close()
		return -1, nil, err
	}
	if connectError != nil {
		cronetConn.Close()
		proxyUDPConn.Close()
		return -1, nil, connectError
	}

	// Duplicate cronetConn's handle for Chromium
	cronetRawConn, err := cronetConn.SyscallConn()
	if err != nil {
		cronetConn.Close()
		proxyUDPConn.Close()
		return -1, nil, err
	}

	var cronetHandle windows.Handle
	var duplicateError error
	err = cronetRawConn.Control(func(fd uintptr) {
		currentProcess, processError := windows.GetCurrentProcess()
		if processError != nil {
			duplicateError = processError
			return
		}
		duplicateError = windows.DuplicateHandle(
			currentProcess,
			windows.Handle(fd),
			currentProcess,
			&cronetHandle,
			0,
			false,
			windows.DUPLICATE_SAME_ACCESS,
		)
	})
	if err != nil {
		cronetConn.Close()
		proxyUDPConn.Close()
		return -1, nil, err
	}
	if duplicateError != nil {
		cronetConn.Close()
		proxyUDPConn.Close()
		return -1, nil, duplicateError
	}

	// Close the Go-side connection (duplicated handle is still valid)
	cronetConn.Close()

	return int(cronetHandle), proxyUDPConn, nil
}

