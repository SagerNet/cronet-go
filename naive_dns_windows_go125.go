//go:build windows && go1.25

package cronet

import "net"

func createPacketSocketPair(forceUDPLoopback bool) (cronetFD int, proxyConn net.PacketConn, err error) {
	// AF_UNIX SOCK_STREAM on Windows is message-mode: each send() is an
	// atomic message and recv() with a buffer smaller than the message
	// returns WSAEMSGSIZE. It cannot safely carry the 2-byte framed packet
	// protocol, so always use a real UDP loopback pair.
	return createUDPLoopbackPair()
}
