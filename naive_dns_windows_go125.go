//go:build windows && go1.25

package cronet

import "net"

func createPacketSocketPair(forceUDPLoopback bool) (cronetFD int, proxyConn net.PacketConn, err error) {
	if forceUDPLoopback {
		return createUDPLoopbackPair()
	}

	// Try Unix socket pair first (Go 1.25+ has FileConn support)
	cronetFD, streamConn, err := createUnixSocketPair()
	if err != nil {
		return createUDPLoopbackPair()
	}

	return cronetFD, newFramedPacketConn(streamConn), nil
}
