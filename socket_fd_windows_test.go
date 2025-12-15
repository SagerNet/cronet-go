//go:build windows

package cronet

import (
	"io"
	"net"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modws2_32 = windows.NewLazySystemDLL("ws2_32.dll")

	procSend = modws2_32.NewProc("send")
	procRecv = modws2_32.NewProc("recv")
)

func winsockSend(socket windows.Handle, buf []byte) (int, error) {
	r1, _, err := procSend.Call(
		uintptr(socket),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		0,
	)
	n := int(r1)
	if n == -1 {
		return 0, err
	}
	return n, nil
}

func winsockRecv(socket windows.Handle, buf []byte) (int, error) {
	r1, _, err := procRecv.Call(
		uintptr(socket),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		0,
	)
	n := int(r1)
	if n == -1 {
		return 0, err
	}
	return n, nil
}

func TestDupSocketFD(t *testing.T) {
	// Create a local TCP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		io.Copy(conn, conn) // echo server
	}()

	// Dial the server
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	// Extract and duplicate the handle
	tcpConn := conn.(*net.TCPConn)
	fd, err := dupSocketFD(tcpConn)
	if err != nil {
		conn.Close()
		t.Fatal(err)
	}

	// Close the original connection
	conn.Close()

	// Verify the duplicated handle is valid
	if fd < 0 {
		t.Fatal("invalid handle returned")
	}

	// Clean up
	windows.CloseHandle(windows.Handle(fd))
}

func TestCreateSocketPair(t *testing.T) {
	fd, conn, err := createSocketPair()
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(windows.Handle(fd))
	defer conn.Close()

	if fd < 0 {
		t.Fatal("invalid handle returned")
	}

	// Test bidirectional communication: fd (socket handle) -> conn
	testData := []byte("hello from fd side")
	done := make(chan error, 1)
	go func() {
		_, err := winsockSend(windows.Handle(fd), testData)
		done <- err
	}()

	buf := make([]byte, len(testData))
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err = io.ReadFull(conn, buf)
	if err != nil {
		t.Fatalf("failed to read from conn: %v", err)
	}
	if string(buf) != string(testData) {
		t.Errorf("data mismatch: got %q, want %q", buf, testData)
	}

	if err := <-done; err != nil {
		t.Fatalf("failed to write to fd: %v", err)
	}

	// Test bidirectional communication: conn -> fd
	testData2 := []byte("hello from conn side")
	go func() {
		conn.Write(testData2)
	}()

	buf2 := make([]byte, len(testData2))
	n, err := winsockRecv(windows.Handle(fd), buf2)
	if err != nil {
		t.Fatalf("failed to read from fd: %v", err)
	}
	if n != len(testData2) || string(buf2[:n]) != string(testData2) {
		t.Errorf("data mismatch: got %q, want %q", buf2[:n], testData2)
	}
}

func TestCreateSocketPair_MultipleCreation(t *testing.T) {
	// Test that we can create multiple socket pairs
	pairs := make([]struct {
		fd   int
		conn net.Conn
	}, 5)

	for i := range pairs {
		fd, conn, err := createSocketPair()
		if err != nil {
			t.Fatalf("failed to create socket pair %d: %v", i, err)
		}
		pairs[i].fd = fd
		pairs[i].conn = conn
	}

	// Clean up
	for _, pair := range pairs {
		windows.CloseHandle(windows.Handle(pair.fd))
		pair.conn.Close()
	}
}
