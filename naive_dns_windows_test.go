//go:build windows

package cronet

import (
	"bytes"
	"encoding/binary"
	"testing"

	"golang.org/x/sys/windows"
)

func TestCreatePacketSocketPair(t *testing.T) {
	fd, conn, err := createPacketSocketPair(false)
	if err != nil {
		t.Fatalf("createPacketSocketPair failed: %v", err)
	}
	defer windows.Closesocket(windows.Handle(fd))
	defer conn.Close()

	if fd <= 0 {
		t.Errorf("expected valid fd, got %d", fd)
	}
}

func TestFramedPacketConn_ReadWrite(t *testing.T) {
	fd, conn, err := createPacketSocketPair(false)
	if err != nil {
		t.Fatalf("createPacketSocketPair failed: %v", err)
	}
	defer windows.Closesocket(windows.Handle(fd))
	defer conn.Close()

	testData := []byte("framed message test")
	_, err = conn.WriteTo(testData, nil)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	buf := make([]byte, 1024)
	var flags uint32
	var bytesReceived uint32
	wsaBuf := windows.WSABuf{Len: uint32(len(buf)), Buf: &buf[0]}
	err = windows.WSARecv(windows.Handle(fd), &wsaBuf, 1, &bytesReceived, &flags, nil, nil)
	if err != nil {
		t.Fatalf("WSARecv failed: %v", err)
	}

	if bytesReceived < 2+uint32(len(testData)) {
		t.Fatalf("expected at least %d bytes, got %d", 2+len(testData), bytesReceived)
	}

	length := binary.BigEndian.Uint16(buf[:2])
	if length != uint16(len(testData)) {
		t.Errorf("expected length %d, got %d", len(testData), length)
	}
	if !bytes.Equal(buf[2:2+length], testData) {
		t.Errorf("expected %q, got %q", testData, buf[2:2+length])
	}
}

func TestFramedPacketConn_BidirectionalCommunication(t *testing.T) {
	fd, conn, err := createPacketSocketPair(false)
	if err != nil {
		t.Fatalf("createPacketSocketPair failed: %v", err)
	}
	defer windows.Closesocket(windows.Handle(fd))
	defer conn.Close()

	testData := []byte("hello from fd")
	frame := make([]byte, 2+len(testData))
	binary.BigEndian.PutUint16(frame[:2], uint16(len(testData)))
	copy(frame[2:], testData)

	wsaBuf := windows.WSABuf{Len: uint32(len(frame)), Buf: &frame[0]}
	var bytesSent uint32
	err = windows.WSASend(windows.Handle(fd), &wsaBuf, 1, &bytesSent, 0, nil, nil)
	if err != nil {
		t.Fatalf("WSASend failed: %v", err)
	}

	buf := make([]byte, 1024)
	n, _, err := conn.ReadFrom(buf)
	if err != nil {
		t.Fatalf("ReadFrom failed: %v", err)
	}
	if !bytes.Equal(buf[:n], testData) {
		t.Errorf("expected %q, got %q", testData, buf[:n])
	}

	testData2 := []byte("hello from conn")
	_, err = conn.WriteTo(testData2, nil)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	var flags uint32
	var bytesReceived uint32
	buf2 := make([]byte, 1024)
	wsaBuf2 := windows.WSABuf{Len: uint32(len(buf2)), Buf: &buf2[0]}
	err = windows.WSARecv(windows.Handle(fd), &wsaBuf2, 1, &bytesReceived, &flags, nil, nil)
	if err != nil {
		t.Fatalf("WSARecv failed: %v", err)
	}

	length := binary.BigEndian.Uint16(buf2[:2])
	if !bytes.Equal(buf2[2:2+length], testData2) {
		t.Errorf("expected %q, got %q", testData2, buf2[2:2+length])
	}
}

func TestFramedPacketConn_MessageBoundary(t *testing.T) {
	fd, conn, err := createPacketSocketPair(false)
	if err != nil {
		t.Fatalf("createPacketSocketPair failed: %v", err)
	}
	defer windows.Closesocket(windows.Handle(fd))
	defer conn.Close()

	messages := [][]byte{
		[]byte("msg1"),
		[]byte("longer message 2"),
		[]byte("m3"),
	}

	for _, msg := range messages {
		_, err = conn.WriteTo(msg, nil)
		if err != nil {
			t.Fatalf("WriteTo failed: %v", err)
		}
	}

	for i, expected := range messages {
		lengthBuf := make([]byte, 2)
		var flags uint32
		var bytesReceived uint32
		wsaBuf := windows.WSABuf{Len: 2, Buf: &lengthBuf[0]}
		err = windows.WSARecv(windows.Handle(fd), &wsaBuf, 1, &bytesReceived, &flags, nil, nil)
		if err != nil {
			t.Fatalf("WSARecv length %d failed: %v", i, err)
		}

		length := binary.BigEndian.Uint16(lengthBuf)
		if length != uint16(len(expected)) {
			t.Errorf("message %d: expected length %d, got %d", i, len(expected), length)
		}

		payload := make([]byte, length)
		wsaBuf2 := windows.WSABuf{Len: uint32(length), Buf: &payload[0]}
		err = windows.WSARecv(windows.Handle(fd), &wsaBuf2, 1, &bytesReceived, &flags, nil, nil)
		if err != nil {
			t.Fatalf("WSARecv payload %d failed: %v", i, err)
		}
		if !bytes.Equal(payload, expected) {
			t.Errorf("message %d: expected %q, got %q", i, expected, payload)
		}
	}
}

func TestFramedPacketConn_MaxSize(t *testing.T) {
	fd, conn, err := createPacketSocketPair(false)
	if err != nil {
		t.Fatalf("createPacketSocketPair failed: %v", err)
	}
	defer windows.Closesocket(windows.Handle(fd))
	defer conn.Close()

	hugePacket := make([]byte, 65536)
	_, err = conn.WriteTo(hugePacket, nil)
	if err == nil {
		t.Error("expected error for packet larger than 65535, got nil")
	}
}
