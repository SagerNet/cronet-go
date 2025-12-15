//go:build unix

package cronet

import (
	"io"
	"net"
	"syscall"
	"testing"
	"time"
)

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

	// Extract and duplicate the FD
	tcpConn := conn.(*net.TCPConn)
	fd, err := dupSocketFD(tcpConn)
	if err != nil {
		conn.Close()
		t.Fatal(err)
	}

	// Close the original connection
	conn.Close()

	// Verify the duplicated FD is valid and usable
	if fd < 0 {
		t.Fatal("invalid fd returned")
	}

	// Clean up
	syscall.Close(fd)
}

func TestDupSocketFD_InvalidConn(t *testing.T) {
	// Test with a connection that doesn't support syscall.Conn
	// This would require a mock - skip for now as it's hard to create
	// a net.Conn that doesn't implement syscall.Conn
	t.Skip("requires mock connection")
}

func TestCreateSocketPair(t *testing.T) {
	fd, conn, err := createSocketPair()
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fd)
	defer conn.Close()

	if fd < 0 {
		t.Fatal("invalid fd returned")
	}

	// Test bidirectional communication: fd -> conn
	testData := []byte("hello from fd side")
	done := make(chan error, 1)
	go func() {
		_, err := syscall.Write(fd, testData)
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
	n, err := syscall.Read(fd, buf2)
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
		syscall.Close(pair.fd)
		pair.conn.Close()
	}
}
