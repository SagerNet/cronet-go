package cronet

import (
	"context"
	"encoding/base64"
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ N.Dialer = (*NaiveClient)(nil)

// QUICCongestionControl represents the QUIC congestion control algorithm.
type QUICCongestionControl string

const (
	// QUICCongestionControlDefault uses Chromium's default (BBR).
	QUICCongestionControlDefault QUICCongestionControl = ""
	// QUICCongestionControlBBR uses BBRv1.
	QUICCongestionControlBBR QUICCongestionControl = "TBBR"
	// QUICCongestionControlBBRv2 uses BBRv2.
	QUICCongestionControlBBRv2 QUICCongestionControl = "B2ON"
	// QUICCongestionControlCubic uses TCP Cubic.
	QUICCongestionControlCubic QUICCongestionControl = "QBIC"
	// QUICCongestionControlReno uses TCP Reno.
	QUICCongestionControlReno QUICCongestionControl = "RENO"
)

type NaiveClient struct {
	ctx                               context.Context
	dialer                            N.Dialer
	serverAddress                     M.Socksaddr
	serverName                        string
	serverURL                         string
	authorization                     string
	extraHeaders                      map[string]string
	trustedRootCertificates           string
	trustedCertificatePublicKeySHA256 [][]byte
	dnsResolver                       DNSResolverFunc
	echEnabled                        bool
	echConfigList                     []byte
	echQueryServerName                string
	echMutex                          sync.RWMutex
	testForceUDPLoopback              bool
	quicEnabled                       bool
	quicCongestionControl             QUICCongestionControl
	concurrency                       int
	counter                           atomic.Uint64
	engine                            Engine
	streamEngine                      StreamEngine
	activeConnections                 sync.WaitGroup
	proxyWaitGroup                    sync.WaitGroup
	proxyCancel                       context.CancelFunc
}

type NaiveClientConfig struct {
	Context                           context.Context
	ServerAddress                     M.Socksaddr
	ServerName                        string
	Username                          string
	Password                          string
	InsecureConcurrency               int
	ExtraHeaders                      map[string]string
	TrustedRootCertificates           string   // PEM format
	TrustedCertificatePublicKeySHA256 [][]byte // SPKI SHA256 hashes
	DNSResolver                       DNSResolverFunc
	Dialer                            N.Dialer

	// ECHEnabled enables Encrypted Client Hello support.
	// When true, Chromium will query HTTPS records from DNS to obtain ECH configs.
	// This works with or without a custom DNSResolver:
	// - With DNSResolver: ECH configs from custom DNS (or ECHConfigList if set)
	// - Without DNSResolver: ECH configs from system DNS
	ECHEnabled bool

	// ECHConfigList is the raw ECH config list in wire format (not PEM).
	// When set, the DNS resolver will inject this into HTTPS record responses
	// for the server name. This allows manual ECH configuration when the
	// upstream DNS doesn't provide ECH configs.
	// If ECH negotiation fails and the server provides retry configs,
	// NaiveClient will automatically update this value internally.
	ECHConfigList []byte

	// ECHQueryServerName overrides the domain name used for ECH HTTPS record queries.
	// If empty, defaults to ServerName.
	// This is useful when the ECH config should be fetched from a different domain
	// than the one used for SNI.
	ECHQueryServerName string

	// TestForceUDPLoopback forces the use of UDP loopback sockets instead of
	// Unix domain sockets for DNS interception. This is for testing only.
	TestForceUDPLoopback bool

	// QUIC enables QUIC protocol and forces its use without TCP/HTTP fallback.
	// The server must support QUIC (sing-box naive with network: udp).
	QUIC bool

	// QUICCongestionControl sets the congestion control algorithm for QUIC.
	// Default is BBR. Only effective when QUIC is enabled.
	QUICCongestionControl QUICCongestionControl
}

func NewNaiveClient(config NaiveClientConfig) (*NaiveClient, error) {
	err := checkLibrary()
	if err != nil {
		return nil, err
	}
	if !config.ServerAddress.IsValid() {
		return nil, E.New("invalid server address")
	}
	if config.DNSResolver == nil {
		return nil, E.New("DNSResolver is required")
	}

	serverName := config.ServerName
	if serverName == "" {
		serverName = config.ServerAddress.AddrString()
	}

	serverURL := &url.URL{
		Scheme: "https",
		Host:   F.ToString(serverName, ":", config.ServerAddress.Port),
	}

	var authorization string
	if config.Username != "" {
		authorization = "Basic " + base64.StdEncoding.EncodeToString(
			[]byte(config.Username+":"+config.Password))
	}

	concurrency := config.InsecureConcurrency
	if concurrency < 1 {
		concurrency = 1
	}

	ctx := config.Context
	if ctx == nil {
		ctx = context.Background()
	}

	dialer := config.Dialer
	if dialer == nil {
		dialer = N.SystemDialer
	}

	return &NaiveClient{
		ctx:                               ctx,
		dialer:                            dialer,
		serverAddress:                     config.ServerAddress,
		serverName:                        serverName,
		serverURL:                         serverURL.String(),
		authorization:                     authorization,
		extraHeaders:                      config.ExtraHeaders,
		trustedRootCertificates:           config.TrustedRootCertificates,
		trustedCertificatePublicKeySHA256: config.TrustedCertificatePublicKeySHA256,
		dnsResolver:                       config.DNSResolver,
		echEnabled:                        config.ECHEnabled,
		echConfigList:                     config.ECHConfigList,
		echQueryServerName:                config.ECHQueryServerName,
		testForceUDPLoopback:              config.TestForceUDPLoopback,
		quicEnabled:                       config.QUIC,
		quicCongestionControl:             config.QUICCongestionControl,
		concurrency:                       concurrency,
	}, nil
}

func (c *NaiveClient) Start() error {
	engine := NewEngine()

	if len(c.trustedCertificatePublicKeySHA256) > 0 {
		if !engine.SetCertVerifierWithPublicKeySHA256(c.trustedCertificatePublicKeySHA256) {
			return E.New("failed to set certificate public key SHA256 verifier")
		}
	} else if c.trustedRootCertificates != "" {
		if !engine.SetTrustedRootCertificates(c.trustedRootCertificates) {
			return E.New("failed to set trusted CA certificates")
		}
	}

	proxyContext, proxyCancel := context.WithCancel(c.ctx)
	c.proxyCancel = proxyCancel

	// Use placeholder address for DNS interception
	dnsServerAddress := M.ParseSocksaddrHostPort("127.0.0.1", 53)
	dnsResolver := c.dnsResolver

	// If ServerName differs from ServerAddress, redirect DNS queries
	if c.serverName != c.serverAddress.AddrString() {
		dnsResolver = wrapDNSResolverForServerRedirect(dnsResolver, c.serverName, c.serverAddress)
	}

	// If ECH is enabled, wrap resolver to inject ECH config into HTTPS records
	if c.echEnabled {
		echQueryServerName := c.echQueryServerName
		if echQueryServerName == "" {
			echQueryServerName = c.serverName
		}
		dnsResolver = wrapDNSResolverWithECH(dnsResolver, c.serverName, echQueryServerName, c.getECHConfigList)
	}

	engine.SetDialer(func(address string, port uint16) int {
		if address == dnsServerAddress.AddrString() && port == dnsServerAddress.Port {
			fd, conn, err := createSocketPair()
			if err != nil {
				return -104 // ERR_CONNECTION_FAILED
			}

			go func() {
				_ = serveDNSStreamConn(proxyContext, conn, dnsResolver)
			}()

			return fd
		}

		conn, err := c.dialer.DialContext(proxyContext, N.NetworkTCP, M.ParseSocksaddrHostPort(address, port))
		if err != nil {
			return mapDialErrorToNetError(err)
		}

		// Fast path: try to extract FD directly from syscall.Conn.
		if syscallConn, ok := conn.(syscall.Conn); ok {
			fd, duplicateError := dupSocketFD(syscallConn)
			if duplicateError == nil {
				conn.Close()
				return fd
			}
		}

		// Best-effort unwrap for common wrappers.
		if tcpConn, ok := N.CastReader[*net.TCPConn](conn); ok {
			fd, duplicateError := dupSocketFD(tcpConn)
			if duplicateError == nil {
				conn.Close()
				return fd
			}
		}

		// Fallback path: create pipe and proxy the connection
		fd, pipeConn, err := createSocketPair()
		if err != nil {
			conn.Close()
			return -104 // ERR_CONNECTION_FAILED
		}

		c.proxyWaitGroup.Add(1)
		go func() {
			defer c.proxyWaitGroup.Done()
			bufio.CopyConn(proxyContext, conn, pipeConn)
			conn.Close()
			pipeConn.Close()
		}()

		return fd
	})

	engine.SetUDPDialer(func(address string, port uint16) (fd int, localAddress string, localPort uint16) {
		// Intercept DNS traffic to the placeholder address
		if address == dnsServerAddress.AddrString() && port == dnsServerAddress.Port {
			fd, conn, err := createPacketSocketPair(c.testForceUDPLoopback)
			if err != nil {
				return -104, "", 0 // ERR_CONNECTION_FAILED
			}

			go func() {
				_ = serveDNSPacketConn(proxyContext, conn, dnsResolver)
			}()

			return fd, address, port
		}

		conn, err := c.dialer.DialContext(proxyContext, N.NetworkUDP, M.ParseSocksaddrHostPort(address, port))
		if err != nil {
			return mapDialErrorToNetError(err), "", 0
		}

		// Try to get local address from connection
		if localAddr := conn.LocalAddr(); localAddr != nil {
			if udpAddr, ok := localAddr.(*net.UDPAddr); ok {
				localAddress = udpAddr.IP.String()
				localPort = uint16(udpAddr.Port)
			}
		}

		if syscallConn, ok := conn.(syscall.Conn); ok {
			fd, duplicateError := dupSocketFD(syscallConn)
			if duplicateError == nil {
				conn.Close()
				return fd, localAddress, localPort
			}
		}

		if udpConn, ok := N.CastReader[*net.UDPConn](conn); ok {
			fd, duplicateError := dupSocketFD(udpConn)
			if duplicateError == nil {
				conn.Close()
				return fd, localAddress, localPort
			}
		}

		// Fallback path: create packet socketpair and proxy the connection
		fd, pipeConn, err := createPacketSocketPair(c.testForceUDPLoopback)
		if err != nil {
			conn.Close()
			return -104, "", 0 // ERR_CONNECTION_FAILED
		}

		c.proxyWaitGroup.Add(1)
		go func() {
			defer c.proxyWaitGroup.Done()
			proxyUDPConnection(proxyContext, conn, pipeConn)
			conn.Close()
			pipeConn.Close()
		}()

		return fd, localAddress, localPort
	})

	params := NewEngineParams()
	if c.quicEnabled {
		params.SetEnableQuic(true)
	} else {
		params.SetEnableHTTP2(true)
	}

	err := params.SetAsyncDNS(true)
	if err != nil {
		return err
	}
	err = params.SetDNSServerOverride([]string{dnsServerAddress.String()})
	if err != nil {
		return err
	}

	if c.echEnabled {
		// Enable HTTPS/SVCB DNS record lookups for ECH support
		err := params.SetUseDnsHttpsSvcb(true)
		if err != nil {
			return err
		}
	}

	if c.quicCongestionControl != "" {
		err := params.SetExperimentalOption("QUIC", map[string]any{
			"connection_options": string(c.quicCongestionControl),
		})
		if err != nil {
			return err
		}
	}

	engine.StartWithParams(params)
	params.Destroy()

	c.engine = engine
	c.streamEngine = engine.StreamEngine()

	return nil
}

func (c *NaiveClient) Engine() Engine {
	return c.engine
}

func (c *NaiveClient) DialEarly(destination M.Socksaddr) (NaiveConn, error) {
	headers := map[string]string{
		"-connect-authority": destination.String(),
		"Padding":            generatePaddingHeader(),
	}
	if c.authorization != "" {
		headers["proxy-authorization"] = c.authorization
	}
	if c.quicEnabled {
		headers["-force-quic"] = "true"
	}
	for key, value := range c.extraHeaders {
		headers[key] = value
	}

	if c.concurrency > 1 {
		concurrencyIndex := int(c.counter.Add(1) % uint64(c.concurrency))
		headers["-network-isolation-key"] = F.ToString("https://pool-", concurrencyIndex, ":443")
	}
	c.activeConnections.Add(1)
	conn := c.streamEngine.CreateConn(true, true)
	err := conn.Start("CONNECT", c.serverURL, headers, 0, false)
	if err != nil {
		c.activeConnections.Done()
		return nil, err
	}
	return &trackedNaiveConn{
		NaiveConn: NewNaiveConn(conn),
		onClose:   c.activeConnections.Done,
	}, nil
}

func (c *NaiveClient) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if N.NetworkName(network) != N.NetworkTCP {
		return nil, os.ErrInvalid
	}
	conn, err := c.DialEarly(destination)
	if err != nil {
		return nil, err
	}
	err = conn.HandshakeContext(ctx)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func (c *NaiveClient) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

func (c *NaiveClient) Close() error {
	if c.proxyCancel != nil {
		c.proxyCancel()
	}
	c.proxyWaitGroup.Wait()
	c.activeConnections.Wait()
	c.engine.Shutdown()
	c.engine.Destroy()
	return nil
}

// getECHConfigList returns the current ECH config list (thread-safe).
// This is used internally by the DNS resolver wrapper.
func (c *NaiveClient) getECHConfigList() []byte {
	c.echMutex.RLock()
	defer c.echMutex.RUnlock()
	return c.echConfigList
}

// ECHConfigList returns the current ECH config list.
func (c *NaiveClient) ECHConfigList() []byte {
	return c.getECHConfigList()
}

// SetECHConfigList updates the ECH configuration for future connections.
// The new config will be injected into DNS HTTPS record responses.
// This is typically called when ECH retry configs are received from the server.
func (c *NaiveClient) SetECHConfigList(echConfigList []byte) {
	c.echMutex.Lock()
	defer c.echMutex.Unlock()
	c.echConfigList = echConfigList
}

// mapDialErrorToNetError maps Go dial errors to Chromium net error codes.
// Returns negative error codes as expected by Chromium.
func mapDialErrorToNetError(err error) int {
	if err == nil {
		return 0
	}

	if errors.Is(err, context.Canceled) {
		return -3 // ERR_ABORTED
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return -118 // ERR_CONNECTION_TIMED_OUT
	}

	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return -118 // ERR_CONNECTION_TIMED_OUT
	}

	// Check for syscall errors first
	var syscallError *os.SyscallError
	if errors.As(err, &syscallError) {
		if errno, ok := syscallError.Err.(syscall.Errno); ok {
			switch errno {
			case syscall.ECONNREFUSED:
				return -102 // ERR_CONNECTION_REFUSED
			case syscall.ETIMEDOUT:
				return -118 // ERR_CONNECTION_TIMED_OUT
			case syscall.ENETUNREACH, syscall.EHOSTUNREACH:
				return -109 // ERR_ADDRESS_UNREACHABLE
			case syscall.ECONNRESET:
				return -101 // ERR_CONNECTION_RESET
			case syscall.ECONNABORTED:
				return -103 // ERR_CONNECTION_ABORTED
			}
		}
	}

	// Check error message patterns
	errorMessage := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errorMessage, "connection refused"):
		return -102 // ERR_CONNECTION_REFUSED
	case strings.Contains(errorMessage, "connection timed out") || strings.Contains(errorMessage, "i/o timeout"):
		return -118 // ERR_CONNECTION_TIMED_OUT
	case strings.Contains(errorMessage, "network is unreachable") || strings.Contains(errorMessage, "no route to host"):
		return -109 // ERR_ADDRESS_UNREACHABLE
	case strings.Contains(errorMessage, "connection reset"):
		return -101 // ERR_CONNECTION_RESET
	case strings.Contains(errorMessage, "connection aborted"):
		return -103 // ERR_CONNECTION_ABORTED
	}

	return -104 // ERR_CONNECTION_FAILED (default)
}

type trackedNaiveConn struct {
	NaiveConn
	onClose   func()
	closeOnce sync.Once
}

func (c *trackedNaiveConn) Close() error {
	c.closeOnce.Do(c.onClose)
	return c.NaiveConn.Close()
}

// proxyUDPConnection proxies UDP packets between an upstream connection and a pipe (socketpair).
// The upstream connection is a connected UDP socket (net.Conn), and the pipe is a PacketConn
// that connects to Chromium via socketpair.
func proxyUDPConnection(ctx context.Context, upstreamConn net.Conn, pipeConn net.PacketConn) {
	// Close both connections when context is cancelled.
	// For datagram socketpairs, closing the peer doesn't unblock ReadFrom.
	go func() {
		<-ctx.Done()
		upstreamConn.Close()
		pipeConn.Close()
	}()

	// Determine if pipeConn is a Unix socketpair (use Write instead of WriteTo)
	var pipeWriter interface{ Write([]byte) (int, error) }
	if unixConn, ok := pipeConn.(*net.UnixConn); ok {
		pipeWriter = unixConn
	}

	// pipe → upstream (Chromium sends data to remote)
	go func() {
		buffer := make([]byte, 64*1024)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if deadlineConn, ok := pipeConn.(interface{ SetReadDeadline(time.Time) error }); ok {
				_ = deadlineConn.SetReadDeadline(time.Now().Add(time.Second))
			}

			n, _, err := pipeConn.ReadFrom(buffer)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				if netError := (*net.OpError)(nil); errors.As(err, &netError) && netError.Timeout() {
					continue
				}
				return
			}

			_, err = upstreamConn.Write(buffer[:n])
			if err != nil {
				return
			}
		}
	}()

	// upstream → pipe (remote sends data to Chromium)
	buffer := make([]byte, 64*1024)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := upstreamConn.SetReadDeadline(time.Now().Add(time.Second)); err == nil {
			// deadline set successfully
		}

		n, err := upstreamConn.Read(buffer)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			if netError := (*net.OpError)(nil); errors.As(err, &netError) && netError.Timeout() {
				continue
			}
			return
		}

		// Write to pipe: use Write() for Unix socketpair, WriteTo() for others
		if pipeWriter != nil {
			_, _ = pipeWriter.Write(buffer[:n])
		} else {
			_, _ = pipeConn.WriteTo(buffer[:n], nil)
		}
	}
}
