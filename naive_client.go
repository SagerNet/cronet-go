package cronet

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type NaiveClientConfig struct {
	Context       context.Context
	ServerAddress M.Socksaddr
	ServerName    string
	Username      string
	Password      string
	Concurrency   int
	ExtraHeaders  map[string]string

	TrustedRootCertificates    string   // PEM format
	CertificatePublicKeySHA256 [][]byte // SPKI SHA256 hashes

	Dialer N.Dialer
}

type NaiveClient struct {
	ctx                        context.Context
	dialer                     N.Dialer
	serverAddress              M.Socksaddr
	serverName                 string
	serverURL                  string
	authorization              string
	extraHeaders               map[string]string
	trustedRootCertificates    string
	certificatePublicKeySHA256 [][]byte
	concurrency                int
	counter                    atomic.Uint64
	engine                     Engine
	streamEngine               StreamEngine
	activeConnections          sync.WaitGroup
	proxyWaitGroup             sync.WaitGroup
	proxyCancel                context.CancelFunc
}

func NewNaiveClient(config NaiveClientConfig) (*NaiveClient, error) {
	err := checkLibrary()
	if err != nil {
		return nil, err
	}
	if !config.ServerAddress.IsValid() {
		return nil, E.New("invalid server address")
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

	concurrency := config.Concurrency
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
		ctx:                        ctx,
		dialer:                     dialer,
		serverAddress:              config.ServerAddress,
		serverName:                 serverName,
		serverURL:                  serverURL.String(),
		authorization:              authorization,
		extraHeaders:               config.ExtraHeaders,
		trustedRootCertificates:    config.TrustedRootCertificates,
		certificatePublicKeySHA256: config.CertificatePublicKeySHA256,
		concurrency:                concurrency,
	}, nil
}

func (c *NaiveClient) Start() error {
	engine := NewEngine()

	if len(c.certificatePublicKeySHA256) > 0 {
		if !engine.SetCertVerifierWithPublicKeySHA256(c.certificatePublicKeySHA256) {
			return E.New("failed to set certificate public key SHA256 verifier")
		}
	} else if c.trustedRootCertificates != "" {
		if !engine.SetTrustedRootCertificates(c.trustedRootCertificates) {
			return E.New("failed to set trusted CA certificates")
		}
	}

	proxyContext, proxyCancel := context.WithCancel(c.ctx)
	c.proxyCancel = proxyCancel

	engine.SetDialer(func(address string, port uint16) int {
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

	params := NewEngineParams()
	params.SetEnableHTTP2(true)

	// Map server hostname to actual IP address for DNS resolution
	experimentalOptions := map[string]any{
		"HostResolverRules": map[string]any{
			"host_resolver_rules": F.ToString("MAP ", c.serverName, " ", c.serverAddress.AddrString()),
		},
	}
	experimentalOptionsJSON, _ := json.Marshal(experimentalOptions)
	params.SetExperimentalOptions(string(experimentalOptionsJSON))

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

func (c *NaiveClient) DialContext(ctx context.Context, destination M.Socksaddr) (NaiveConn, error) {
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
