//go:build windows

package cronet

import (
	"net"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

func createUDPLoopbackPair() (cronetFD int, proxyConn net.PacketConn, err error) {
	proxyUDPConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		return -1, nil, E.Cause(err, "create UDP loopback listener")
	}
	proxyLocalAddress := proxyUDPConn.LocalAddr().(*net.UDPAddr)

	cronetSocket, err := windows.WSASocket(
		windows.AF_INET,
		windows.SOCK_DGRAM,
		windows.IPPROTO_UDP,
		nil,
		0,
		windows.WSA_FLAG_OVERLAPPED|windows.WSA_FLAG_NO_HANDLE_INHERIT,
	)
	if err != nil {
		proxyUDPConn.Close()
		return -1, nil, E.Cause(err, "create UDP loopback socket")
	}

	proxySockaddr := &windows.SockaddrInet4{Port: proxyLocalAddress.Port}
	copy(proxySockaddr.Addr[:], proxyLocalAddress.IP.To4())
	if err = windows.Connect(cronetSocket, proxySockaddr); err != nil {
		_ = windows.Closesocket(cronetSocket)
		proxyUDPConn.Close()
		return -1, nil, E.Cause(err, "connect UDP loopback socket")
	}

	cronetSockaddr, err := windows.Getsockname(cronetSocket)
	if err != nil {
		_ = windows.Closesocket(cronetSocket)
		proxyUDPConn.Close()
		return -1, nil, E.Cause(err, "get UDP loopback socket address")
	}
	cronetLocalAddress, ok := cronetSockaddr.(*windows.SockaddrInet4)
	if !ok {
		_ = windows.Closesocket(cronetSocket)
		proxyUDPConn.Close()
		return -1, nil, E.New("unexpected UDP loopback socket address family")
	}

	proxyRawConn, err := proxyUDPConn.SyscallConn()
	if err != nil {
		_ = windows.Closesocket(cronetSocket)
		proxyUDPConn.Close()
		return -1, nil, E.Cause(err, "get UDP listener syscall conn")
	}

	var connectError error
	err = proxyRawConn.Control(func(fd uintptr) {
		sockaddr := &windows.SockaddrInet4{Port: cronetLocalAddress.Port}
		sockaddr.Addr = cronetLocalAddress.Addr
		connectError = windows.Connect(windows.Handle(fd), sockaddr)
	})
	if err != nil {
		_ = windows.Closesocket(cronetSocket)
		proxyUDPConn.Close()
		return -1, nil, E.Cause(err, "control UDP listener")
	}
	if connectError != nil {
		_ = windows.Closesocket(cronetSocket)
		proxyUDPConn.Close()
		return -1, nil, E.Cause(connectError, "connect UDP listener")
	}

	return int(cronetSocket), proxyUDPConn, nil
}
