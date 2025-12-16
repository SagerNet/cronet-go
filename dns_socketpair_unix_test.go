//go:build unix

package cronet

import (
	"syscall"
	"testing"
)

func TestCreatePacketSocketPair(t *testing.T) {
	fd, conn, err := createPacketSocketPair(false)
	if err != nil {
		t.Fatalf("createPacketSocketPair failed: %v", err)
	}
	defer syscall.Close(fd)
	defer conn.Close()

	if fd <= 0 {
		t.Errorf("expected valid fd, got %d", fd)
	}
}

func TestCreatePacketSocketPair_BidirectionalCommunication(t *testing.T) {
	fd, conn, err := createPacketSocketPair(false)
	if err != nil {
		t.Fatalf("createPacketSocketPair failed: %v", err)
	}
	defer syscall.Close(fd)
	defer conn.Close()

	// fd → conn (datagram boundary preserved)
	testData := []byte("hello from fd")
	_, err = syscall.Write(fd, testData)
	if err != nil {
		t.Fatalf("write to fd failed: %v", err)
	}

	buf := make([]byte, 1024)
	n, _, err := conn.ReadFrom(buf)
	if err != nil {
		t.Fatalf("read from conn failed: %v", err)
	}
	if string(buf[:n]) != string(testData) {
		t.Errorf("expected %q, got %q", testData, buf[:n])
	}

	// conn → fd: For Unix socketpairs, use Write() via net.Conn interface
	// (same as serveDNSPacketConn does when remoteAddress is nil)
	streamConn, ok := conn.(interface{ Write([]byte) (int, error) })
	if !ok {
		t.Fatal("PacketConn should implement Write for socketpair")
	}
	testData2 := []byte("hello from conn")
	_, err = streamConn.Write(testData2)
	if err != nil {
		t.Fatalf("write to conn failed: %v", err)
	}

	n, err = syscall.Read(fd, buf)
	if err != nil {
		t.Fatalf("read from fd failed: %v", err)
	}
	if string(buf[:n]) != string(testData2) {
		t.Errorf("expected %q, got %q", testData2, buf[:n])
	}
}

func TestCreatePacketSocketPair_MessageBoundary(t *testing.T) {
	fd, conn, err := createPacketSocketPair(false)
	if err != nil {
		t.Fatalf("createPacketSocketPair failed: %v", err)
	}
	defer syscall.Close(fd)
	defer conn.Close()

	// Send multiple different-sized messages
	messages := [][]byte{
		[]byte("short"),
		make([]byte, 512),
		make([]byte, 1400),
	}
	// Fill with recognizable patterns
	for i := range messages[1] {
		messages[1][i] = byte(i % 256)
	}
	for i := range messages[2] {
		messages[2][i] = byte((i * 7) % 256)
	}

	for _, msg := range messages {
		_, err = syscall.Write(fd, msg)
		if err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}

	// Verify each message boundary is preserved
	for i, expected := range messages {
		buf := make([]byte, 2048)
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			t.Fatalf("read %d failed: %v", i, err)
		}
		if n != len(expected) {
			t.Errorf("message %d: expected length %d, got %d", i, len(expected), n)
		}
		if string(buf[:n]) != string(expected) {
			t.Errorf("message %d: content mismatch", i)
		}
	}
}

func TestCreateUDPLoopbackPair(t *testing.T) {
	fd, conn, err := createUDPLoopbackPair()
	if err != nil {
		t.Fatalf("createUDPLoopbackPair failed: %v", err)
	}
	defer syscall.Close(fd)
	defer conn.Close()

	if fd <= 0 {
		t.Errorf("expected valid fd, got %d", fd)
	}
}

func TestCreateUDPLoopbackPair_BidirectionalCommunication(t *testing.T) {
	fd, conn, err := createUDPLoopbackPair()
	if err != nil {
		t.Fatalf("createUDPLoopbackPair failed: %v", err)
	}
	defer syscall.Close(fd)
	defer conn.Close()

	// Set fd to blocking mode (it may be non-blocking after dup)
	if err := syscall.SetNonblock(fd, false); err != nil {
		t.Fatalf("failed to set blocking mode: %v", err)
	}

	// fd → conn
	testData := []byte("hello from fd via UDP loopback")
	_, err = syscall.Write(fd, testData)
	if err != nil {
		t.Fatalf("write to fd failed: %v", err)
	}

	buf := make([]byte, 1024)
	n, remoteAddr, err := conn.ReadFrom(buf)
	if err != nil {
		t.Fatalf("read from conn failed: %v", err)
	}
	if string(buf[:n]) != string(testData) {
		t.Errorf("expected %q, got %q", testData, buf[:n])
	}
	if remoteAddr == nil {
		t.Error("expected non-nil remote address for UDP loopback")
	}

	// conn → fd: Use Write (both sockets are connected to each other)
	streamConn, ok := conn.(interface{ Write([]byte) (int, error) })
	if !ok {
		t.Fatal("connected UDP socket should implement Write")
	}
	testData2 := []byte("hello from conn via UDP loopback")
	_, err = streamConn.Write(testData2)
	if err != nil {
		t.Fatalf("write to conn failed: %v", err)
	}

	n, err = syscall.Read(fd, buf)
	if err != nil {
		t.Fatalf("read from fd failed: %v", err)
	}
	if string(buf[:n]) != string(testData2) {
		t.Errorf("expected %q, got %q", testData2, buf[:n])
	}
}

func TestCreatePacketSocketPair_ForceUDPLoopback(t *testing.T) {
	// Test that forceUDPLoopback=true returns UDP loopback pair
	fd, conn, err := createPacketSocketPair(true)
	if err != nil {
		t.Fatalf("createPacketSocketPair(true) failed: %v", err)
	}
	defer syscall.Close(fd)
	defer conn.Close()

	// Verify it's a UDP connection by checking that ReadFrom returns a remote address
	testData := []byte("test UDP loopback via forceUDPLoopback")
	_, err = syscall.Write(fd, testData)
	if err != nil {
		t.Fatalf("write to fd failed: %v", err)
	}

	buf := make([]byte, 1024)
	n, remoteAddr, err := conn.ReadFrom(buf)
	if err != nil {
		t.Fatalf("read from conn failed: %v", err)
	}
	if string(buf[:n]) != string(testData) {
		t.Errorf("expected %q, got %q", testData, buf[:n])
	}
	// UDP loopback returns non-nil remote address, Unix socketpair returns nil
	if remoteAddr == nil {
		t.Error("forceUDPLoopback=true should return UDP loopback (non-nil remote address)")
	}
}
