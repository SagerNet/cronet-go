package test

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	cronet "github.com/sagernet/cronet-go"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	mDNS "github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

// localhostDNSResolver returns a DNS resolver that resolves all A/AAAA queries to 127.0.0.1.
func localhostDNSResolver(t *testing.T) cronet.DNSResolverFunc {
	return func(ctx context.Context, request *mDNS.Msg) *mDNS.Msg {
		t.Logf("DNS resolver called for: %v", request.Question)
		response := new(mDNS.Msg)
		response.SetReply(request)
		for _, question := range request.Question {
			switch question.Qtype {
			case mDNS.TypeA:
				t.Logf("Resolving %s to 127.0.0.1", question.Name)
				response.Answer = append(response.Answer, &mDNS.A{
					Hdr: mDNS.RR_Header{
						Name:   question.Name,
						Rrtype: mDNS.TypeA,
						Class:  mDNS.ClassINET,
						Ttl:    300,
					},
					A: net.ParseIP("127.0.0.1"),
				})
			case mDNS.TypeAAAA:
				t.Logf("Resolving %s to ::1", question.Name)
				response.Answer = append(response.Answer, &mDNS.AAAA{
					Hdr: mDNS.RR_Header{
						Name:   question.Name,
						Rrtype: mDNS.TypeAAAA,
						Class:  mDNS.ClassINET,
						Ttl:    300,
					},
					AAAA: net.ParseIP("::1"),
				})
			}
		}
		return response
	}
}

func localhostDNSResolverWithHTTPSResponse(t *testing.T, servicePort uint16, applicationProtocols []string) cronet.DNSResolverFunc {
	baseResolver := localhostDNSResolver(t)
	return func(ctx context.Context, request *mDNS.Msg) *mDNS.Msg {
		response := baseResolver(ctx, request)
		if len(request.Question) == 0 {
			return response
		}
		question := request.Question[0]
		if question.Qtype != mDNS.TypeHTTPS {
			return response
		}
		if response == nil {
			response = new(mDNS.Msg)
			response.SetReply(request)
		}
		targetName := question.Name
		if strings.HasPrefix(targetName, "_") {
			parts := strings.SplitN(targetName, "._https.", 2)
			if len(parts) == 2 {
				targetName = parts[1]
			}
		}
		httpsRecord := &mDNS.HTTPS{
			SVCB: mDNS.SVCB{
				Hdr: mDNS.RR_Header{
					Name:   question.Name,
					Rrtype: mDNS.TypeHTTPS,
					Class:  mDNS.ClassINET,
					Ttl:    300,
				},
				Priority: 1,
				Target:   targetName,
			},
		}
		if servicePort != 0 {
			httpsRecord.Value = append(httpsRecord.Value, &mDNS.SVCBPort{Port: servicePort})
		}
		if len(applicationProtocols) > 0 {
			httpsRecord.Value = append(httpsRecord.Value, &mDNS.SVCBAlpn{Alpn: applicationProtocols})
		}
		httpsRecord.Value = append(httpsRecord.Value, &mDNS.SVCBIPv4Hint{Hint: []net.IP{net.ParseIP("127.0.0.1")}})
		httpsRecord.Value = append(httpsRecord.Value, &mDNS.SVCBIPv6Hint{Hint: []net.IP{net.ParseIP("::1")}})
		response.Answer = append(response.Answer, httpsRecord)
		return response
	}
}

// wrappedConn wraps a net.Conn but is NOT a *net.TCPConn,
// forcing the fallback path (socket pair proxy) in NaiveClient.
type wrappedConn struct {
	net.Conn
}

func (w *wrappedConn) SyscallConn() (syscall.RawConn, error) {
	if conn, ok := w.Conn.(syscall.Conn); ok {
		return conn.SyscallConn()
	}
	return nil, syscall.EINVAL
}

// trackingDialer tracks dial calls and can wrap connections.
type trackingDialer struct {
	underlying N.Dialer
	dialCount  atomic.Int64
	wrapConn   bool // if true, wrap connections to force fallback path
}

func (d *trackingDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	d.dialCount.Add(1)
	conn, err := d.underlying.DialContext(ctx, network, destination)
	if err != nil {
		return nil, err
	}
	if d.wrapConn {
		return &wrappedConn{Conn: conn}, nil
	}
	return conn, nil
}

func (d *trackingDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return d.underlying.ListenPacket(ctx, destination)
}

// errorDialer always returns errors for testing error handling.
type errorDialer struct {
	err error
}

func (d *errorDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return nil, d.err
}

func (d *errorDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, d.err
}

// TestNaiveCustomDialer verifies that a custom dialer is properly used.
func TestNaiveCustomDialer(t *testing.T) {
	env := setupTestEnv(t)
	startEchoServer(t, 17000)

	dialer := &trackingDialer{
		underlying: N.SystemDialer,
		wrapConn:   false,
	}

	client := env.newNaiveClient(t, cronet.NaiveClientOptions{
		Dialer:      dialer,
		DNSResolver: localhostDNSResolver(t),
	})

	// Make a connection
	conn, err := client.DialEarly(M.ParseSocksaddrHostPort("127.0.0.1", 17000))
	require.NoError(t, err)
	defer conn.Close()

	// Verify connection works (this triggers the actual TCP connection)
	testData := []byte("Custom dialer test!")
	_, err = conn.Write(testData)
	require.NoError(t, err)

	buf := make([]byte, len(testData))
	_, err = io.ReadFull(conn, buf)
	require.NoError(t, err)
	require.Equal(t, testData, buf)

	// Verify dialer was called (check after I/O since connection is established lazily)
	require.Greater(t, dialer.dialCount.Load(), int64(0), "custom dialer should have been called")
}

// TestNaivePipeProxy tests the socket pair proxy fallback path.
// This is triggered when the connection is not a *net.TCPConn.
func TestNaivePipeProxy(t *testing.T) {
	env := setupTestEnv(t)
	startEchoServer(t, 17001)

	dialer := &trackingDialer{
		underlying: N.SystemDialer,
		wrapConn:   true, // Force wrapped connections to trigger fallback path
	}

	client := env.newNaiveClient(t, cronet.NaiveClientOptions{
		Dialer:      dialer,
		DNSResolver: localhostDNSResolver(t),
	})

	// Make a connection through the proxy path
	conn, err := client.DialEarly(M.ParseSocksaddrHostPort("127.0.0.1", 17001))
	require.NoError(t, err)
	defer conn.Close()

	// Verify data transfer works through the proxy path
	testData := []byte("Pipe proxy test data!")
	_, err = conn.Write(testData)
	require.NoError(t, err)

	buf := make([]byte, len(testData))
	_, err = io.ReadFull(conn, buf)
	require.NoError(t, err)
	require.Equal(t, testData, buf)

	// Test multiple round trips
	for i := 0; i < 10; i++ {
		data := []byte("Round trip " + string(rune('0'+i)))
		_, err = conn.Write(data)
		require.NoError(t, err)

		readBuf := make([]byte, len(data))
		_, err = io.ReadFull(conn, readBuf)
		require.NoError(t, err)
		require.Equal(t, data, readBuf)
	}
}

// TestNaiveDialError tests error handling when dial fails.
func TestNaiveDialError(t *testing.T) {
	env := setupTestEnv(t)

	testCases := []struct {
		name string
		err  error
	}{
		{"connection refused", errors.New("connection refused")},
		{"timeout", errors.New("i/o timeout")},
		{"network unreachable", errors.New("network is unreachable")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := env.newNaiveClient(t, cronet.NaiveClientOptions{
				Dialer: &errorDialer{err: tc.err},
			})

			// Attempting to dial should fail
			conn, err := client.DialEarly(M.ParseSocksaddrHostPort("127.0.0.1", 17002))
			require.NoError(t, err)
			err = conn.Handshake()
			require.Error(t, err, "expected dial to fail with %s", tc.name)
			conn.Close()
		})
	}
}

// TestNaiveLargeTransfer tests data integrity with large transfers.
func TestNaiveLargeTransfer(t *testing.T) {
	env := setupTestEnv(t)
	startEchoServer(t, 17003)

	client := env.newNaiveClient(t, cronet.NaiveClientOptions{
		DNSResolver: localhostDNSResolver(t),
	})

	conn, err := client.DialEarly(M.ParseSocksaddrHostPort("127.0.0.1", 17003))
	require.NoError(t, err)
	defer conn.Close()

	// Generate 1MB of random data
	const dataSize = 1024 * 1024
	testData := make([]byte, dataSize)
	_, err = rand.Read(testData)
	require.NoError(t, err)

	// Write in background
	writeDone := make(chan error, 1)
	go func() {
		_, err := conn.Write(testData)
		writeDone <- err
	}()

	// Read all data back
	receivedData := make([]byte, dataSize)
	_, err = io.ReadFull(conn, receivedData)
	require.NoError(t, err)

	// Wait for write to complete
	require.NoError(t, <-writeDone)

	// Verify data integrity
	require.True(t, bytes.Equal(testData, receivedData), "data mismatch in large transfer")
}

// TestNaiveRapidOpenClose tests stability with rapid connection open/close cycles.
func TestNaiveRapidOpenClose(t *testing.T) {
	env := setupTestEnv(t)
	startEchoServer(t, 17004)

	client := env.newNaiveClient(t, cronet.NaiveClientOptions{
		DNSResolver: localhostDNSResolver(t),
	})

	const iterations = 50

	for i := 0; i < iterations; i++ {
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			conn, err := client.DialContext(ctx, N.NetworkTCP, M.ParseSocksaddrHostPort("127.0.0.1", 17004))
			if err != nil {
				t.Logf("iteration %d: dial failed (acceptable): %v", i, err)
				return
			}

			// Quick write/read cycle
			testData := []byte("rapid test")
			conn.Write(testData)

			buf := make([]byte, len(testData))
			io.ReadFull(conn, buf)

			conn.Close()
		}()
	}
}

// TestNaiveGracefulShutdown tests that Close() properly waits for all connections.
func TestNaiveGracefulShutdown(t *testing.T) {
	env := setupTestEnv(t)
	startEchoServer(t, 17005)

	// Use wrapped dialer to force proxy path which has more cleanup work
	dialer := &trackingDialer{
		underlying: N.SystemDialer,
		wrapConn:   true,
	}

	config := cronet.NaiveClientOptions{
		ServerAddress: M.ParseSocksaddrHostPort("127.0.0.1", naiveServerPort),
		ServerName:    "example.org",
		Username:      "test",
		Password:      "test",
		Dialer:        dialer,
		DNSResolver:   localhostDNSResolver(t),
	}
	config.TrustedRootCertificates = string(env.caPEM)

	client, err := cronet.NewNaiveClient(config)
	require.NoError(t, err)
	require.NoError(t, client.Start())

	const connectionCount = 5
	var wg sync.WaitGroup
	conns := make([]net.Conn, connectionCount)

	// Open multiple connections
	for i := 0; i < connectionCount; i++ {
		conn, err := client.DialEarly(M.ParseSocksaddrHostPort("127.0.0.1", 17005))
		require.NoError(t, err)
		conns[i] = conn

		// Start background activity on each connection
		wg.Add(1)
		go func(c net.Conn, idx int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				data := []byte("activity")
				if _, err := c.Write(data); err != nil {
					return
				}
				buf := make([]byte, len(data))
				if _, err := io.ReadFull(c, buf); err != nil {
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}(conn, i)
	}

	// Wait a bit for activity to start
	time.Sleep(50 * time.Millisecond)

	// Close all connections first
	for _, conn := range conns {
		conn.Close()
	}

	// Wait for goroutines to finish
	wg.Wait()

	// Now close the client - this should complete without hanging
	closeDone := make(chan struct{})
	go func() {
		client.Close()
		close(closeDone)
	}()

	select {
	case <-closeDone:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("client.Close() timed out - possible resource leak")
	}
}

// TestNaivePipeProxyMultipleConnections tests multiple simultaneous connections
// through the proxy path.
func TestNaivePipeProxyMultipleConnections(t *testing.T) {
	env := setupTestEnv(t)
	startEchoServer(t, 17006)

	dialer := &trackingDialer{
		underlying: N.SystemDialer,
		wrapConn:   true,
	}

	client := env.newNaiveClient(t, cronet.NaiveClientOptions{
		Dialer:      dialer,
		DNSResolver: localhostDNSResolver(t),
	})

	const connectionCount = 10
	var wg sync.WaitGroup
	errChannel := make(chan error, connectionCount)

	for i := 0; i < connectionCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			conn, err := client.DialEarly(M.ParseSocksaddrHostPort("127.0.0.1", 17006))
			if err != nil {
				errChannel <- err
				return
			}
			defer conn.Close()

			// Send unique data
			testData := make([]byte, 100)
			for j := range testData {
				testData[j] = byte(idx)
			}

			_, err = conn.Write(testData)
			if err != nil {
				errChannel <- err
				return
			}

			buf := make([]byte, len(testData))
			_, err = io.ReadFull(conn, buf)
			if err != nil {
				errChannel <- err
				return
			}

			if !bytes.Equal(testData, buf) {
				errChannel <- errors.New("data mismatch")
				return
			}
		}(i)
	}

	wg.Wait()
	close(errChannel)

	for err := range errChannel {
		t.Errorf("connection error: %v", err)
	}
}

// TestDNSTCFallbackToTCP verifies that when UDP DNS returns TC (Truncated),
// Chromium falls back to TCP DNS and our TCP DNS interception works.
func TestDNSTCFallbackToTCP(t *testing.T) {
	env := setupTestEnv(t)
	startEchoServer(t, 17007)

	var dnsCallCount atomic.Int64

	// DNS resolver that returns a large response (> 512 bytes)
	// This will trigger TC on UDP, forcing fallback to TCP
	largeDNSResolver := func(ctx context.Context, request *mDNS.Msg) *mDNS.Msg {
		dnsCallCount.Add(1)
		count := dnsCallCount.Load()
		t.Logf("DNS resolver called (call #%d): %v", count, request.Question)

		response := new(mDNS.Msg)
		response.SetReply(request)

		for _, question := range request.Question {
			if question.Qtype == mDNS.TypeA {
				response.Answer = append(response.Answer, &mDNS.A{
					Hdr: mDNS.RR_Header{
						Name:   question.Name,
						Rrtype: mDNS.TypeA,
						Class:  mDNS.ClassINET,
						Ttl:    300,
					},
					A: net.ParseIP("127.0.0.1"),
				})
			}
		}

		// Add many TXT records to exceed 512 bytes
		for i := 0; i < 20; i++ {
			response.Extra = append(response.Extra, &mDNS.TXT{
				Hdr: mDNS.RR_Header{
					Name:   request.Question[0].Name,
					Rrtype: mDNS.TypeTXT,
					Class:  mDNS.ClassINET,
					Ttl:    300,
				},
				Txt: []string{strings.Repeat("x", 50)},
			})
		}
		return response
	}

	serverAddress := M.ParseSocksaddrHostPort("proxy.invalid", naiveServerPort)
	client := env.newNaiveClient(t, cronet.NaiveClientOptions{
		ServerAddress: serverAddress,
		DNSResolver:   largeDNSResolver,
	})

	// Make a connection - this will trigger DNS resolution
	conn, err := client.DialEarly(M.ParseSocksaddrHostPort("127.0.0.1", 17007))
	require.NoError(t, err)
	defer conn.Close()

	// Verify connection works
	testData := []byte("TC fallback test!")
	_, err = conn.Write(testData)
	require.NoError(t, err)

	buf := make([]byte, len(testData))
	_, err = io.ReadFull(conn, buf)
	require.NoError(t, err)
	require.Equal(t, testData, buf)

	// DNS should be called at least twice (UDP with TC, then TCP)
	require.GreaterOrEqual(t, dnsCallCount.Load(), int64(2),
		"DNS resolver should be called at least twice (UDP TC + TCP fallback)")
	t.Logf("DNS resolver was called %d times", dnsCallCount.Load())
}

// TestDNSInterceptionUDPLoopbackFallback verifies that DNS interception works
// when forcing the UDP loopback socket pair fallback path.
// This tests the path used on all platforms when Unix domain sockets are unavailable.
func TestDNSInterceptionUDPLoopbackFallback(t *testing.T) {
	env := setupTestEnv(t)
	startEchoServer(t, 17008)

	var dnsCallCount atomic.Int64

	countingResolver := func(ctx context.Context, request *mDNS.Msg) *mDNS.Msg {
		dnsCallCount.Add(1)
		t.Logf("UDP loopback DNS resolver called: %v", request.Question)
		response := new(mDNS.Msg)
		response.SetReply(request)
		for _, question := range request.Question {
			if question.Qtype == mDNS.TypeA {
				response.Answer = append(response.Answer, &mDNS.A{
					Hdr: mDNS.RR_Header{
						Name:   question.Name,
						Rrtype: mDNS.TypeA,
						Class:  mDNS.ClassINET,
						Ttl:    300,
					},
					A: net.ParseIP("127.0.0.1"),
				})
			}
		}
		return response
	}

	serverAddress := M.ParseSocksaddrHostPort("proxy.invalid", naiveServerPort)
	client := env.newNaiveClient(t, cronet.NaiveClientOptions{
		ServerAddress:        serverAddress,
		DNSResolver:          countingResolver,
		TestForceUDPLoopback: true, // Force UDP loopback path
	})

	// Make a connection - this will trigger DNS resolution through UDP loopback
	conn, err := client.DialEarly(M.ParseSocksaddrHostPort("127.0.0.1", 17008))
	require.NoError(t, err)
	defer conn.Close()

	// Verify connection works
	testData := []byte("UDP loopback fallback test!")
	_, err = conn.Write(testData)
	require.NoError(t, err)

	buf := make([]byte, len(testData))
	_, err = io.ReadFull(conn, buf)
	require.NoError(t, err)
	require.Equal(t, testData, buf)

	// Verify DNS resolver was called
	require.Greater(t, dnsCallCount.Load(), int64(0),
		"DNS resolver should have been called through UDP loopback path")
	t.Logf("DNS resolver was called %d times via UDP loopback", dnsCallCount.Load())
}

// TestDNSInterceptionDefaultPath verifies that DNS interception works
// with the default platform-specific socketpair implementation.
// - Unix: AF_UNIX SOCK_DGRAM socketpair
// - Windows: AF_UNIX SOCK_STREAM + length-prefix framing
func TestDNSInterceptionDefaultPath(t *testing.T) {
	env := setupTestEnv(t)
	startEchoServer(t, 17009)

	var dnsCallCount atomic.Int64

	countingResolver := func(ctx context.Context, request *mDNS.Msg) *mDNS.Msg {
		dnsCallCount.Add(1)
		t.Logf("Default path DNS resolver called: %v", request.Question)
		response := new(mDNS.Msg)
		response.SetReply(request)
		for _, question := range request.Question {
			if question.Qtype == mDNS.TypeA {
				response.Answer = append(response.Answer, &mDNS.A{
					Hdr: mDNS.RR_Header{
						Name:   question.Name,
						Rrtype: mDNS.TypeA,
						Class:  mDNS.ClassINET,
						Ttl:    300,
					},
					A: net.ParseIP("127.0.0.1"),
				})
			}
		}
		return response
	}

	serverAddress := M.ParseSocksaddrHostPort("proxy.invalid", naiveServerPort)
	client := env.newNaiveClient(t, cronet.NaiveClientOptions{
		ServerAddress:        serverAddress,
		DNSResolver:          countingResolver,
		TestForceUDPLoopback: false, // Use default path (Unix SOCK_DGRAM or Windows framed)
	})

	// Make a connection - this will trigger DNS resolution through default path
	conn, err := client.DialEarly(M.ParseSocksaddrHostPort("127.0.0.1", 17009))
	require.NoError(t, err)
	defer conn.Close()

	// Verify connection works
	testData := []byte("Default path DNS interception test!")
	_, err = conn.Write(testData)
	require.NoError(t, err)

	buf := make([]byte, len(testData))
	_, err = io.ReadFull(conn, buf)
	require.NoError(t, err)
	require.Equal(t, testData, buf)

	// Verify DNS resolver was called
	require.Greater(t, dnsCallCount.Load(), int64(0),
		"DNS resolver should have been called through default path")
	t.Logf("DNS resolver was called %d times via default path", dnsCallCount.Load())
}

// TestNaiveInsecureConcurrencySessionCount verifies that InsecureConcurrency
// actually creates multiple HTTP/2 sessions (regression test for
// PartitionConnectionsByNetworkIsolationKey feature)
func TestNaiveInsecureConcurrencySessionCount(t *testing.T) {
	env := setupTestEnv(t)
	client := env.newNaiveClient(t, cronet.NaiveClientOptions{
		InsecureConcurrency: 3,
		DNSResolver:         localhostDNSResolver(t),
	})

	// Start echo servers for each connection
	const connectionCount = 6
	for i := 0; i < connectionCount; i++ {
		startEchoServer(t, uint16(18000+i))
	}

	// Start NetLog
	netLogPath := filepath.Join(t.TempDir(), "netlog.json")
	require.True(t, client.Engine().StartNetLogToFile(netLogPath, true),
		"Failed to start NetLog")

	// Send multiple sequential connections to trigger round-robin
	for i := 0; i < connectionCount; i++ {
		conn, err := client.DialEarly(M.ParseSocksaddrHostPort("127.0.0.1", uint16(18000+i)))
		require.NoError(t, err)

		testData := []byte("test")
		_, err = conn.Write(testData)
		require.NoError(t, err)

		buf := make([]byte, len(testData))
		_, err = io.ReadFull(conn, buf)
		require.NoError(t, err)

		conn.Close()
	}

	// Stop NetLog and read results
	client.Engine().StopNetLog()

	logContent, err := os.ReadFile(netLogPath)
	require.NoError(t, err)
	logStr := string(logContent)

	// Count HTTP2_SESSION_INITIALIZED events (type 249 in NetLog)
	sessionCount := strings.Count(logStr, `"type":249`)
	require.GreaterOrEqual(t, sessionCount, 3,
		"Expected at least 3 HTTP/2 sessions with InsecureConcurrency=3, got %d. NetLog: %s",
		sessionCount, netLogPath)
}
