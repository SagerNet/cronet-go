package cronet

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/url"
	"sync/atomic"

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
	tcpForwarder               net.Listener
}

func NewNaiveClient(config NaiveClientConfig) (*NaiveClient, error) {
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
	tcpForwarder, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return E.Cause(err, "create tcp forwarder")
	}
	forwarderPort := M.SocksaddrFromNet(tcpForwarder.Addr()).Port

	engine := NewEngine()

	if len(c.certificatePublicKeySHA256) > 0 {
		if !engine.SetCertVerifierWithPublicKeySHA256(c.certificatePublicKeySHA256) {
			tcpForwarder.Close()
			return E.New("failed to set certificate public key SHA256 verifier")
		}
	} else if c.trustedRootCertificates != "" {
		if !engine.SetTrustedRootCertificates(c.trustedRootCertificates) {
			tcpForwarder.Close()
			return E.New("failed to set trusted CA certificates")
		}
	}

	params := NewEngineParams()
	params.SetEnableHTTP2(true)

	experimentalOptions := map[string]any{
		"HostResolverRules": map[string]any{
			"host_resolver_rules": F.ToString("MAP ", c.serverName, ":", c.serverAddress.Port, " 127.0.0.1:", forwarderPort),
		},
	}
	experimentalOptionsJSON, _ := json.Marshal(experimentalOptions)
	params.SetExperimentalOptions(string(experimentalOptionsJSON))

	engine.StartWithParams(params)
	params.Destroy()

	c.engine = engine
	c.streamEngine = engine.StreamEngine()
	c.tcpForwarder = tcpForwarder

	go c.forwardLoop()

	return nil
}

func (c *NaiveClient) Engine() Engine {
	return c.engine
}

func (c *NaiveClient) DialContext(ctx context.Context, destination M.Socksaddr) (net.Conn, error) {
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

	concurrencyIndex := int(c.counter.Add(1) % uint64(c.concurrency))
	conn := c.streamEngine.CreateConn(true, false, concurrencyIndex)
	err := conn.Start("CONNECT", c.serverURL, headers, 0, false)
	if err != nil {
		return nil, err
	}

	return NewNaiveConn(conn), nil
}

func (c *NaiveClient) Close() error {
	c.tcpForwarder.Close()
	c.engine.Shutdown()
	c.engine.Destroy()
	return nil
}

func (c *NaiveClient) forwardLoop() {
	for {
		conn, err := c.tcpForwarder.Accept()
		if err != nil {
			return
		}
		go c.forwardConnection(conn)
	}
}

func (c *NaiveClient) forwardConnection(conn net.Conn) {
	serverConn, err := c.dialer.DialContext(c.ctx, N.NetworkTCP, c.serverAddress)
	if err != nil {
		conn.Close()
		return
	}
	bufio.CopyConn(c.ctx, conn, serverConn)
}
