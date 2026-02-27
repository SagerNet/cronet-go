package test

import (
	"bytes"
	"context"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cronet "github.com/sagernet/cronet-go"
	M "github.com/sagernet/sing/common/metadata"

	mDNS "github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

func startNaiveQUICServer(t *testing.T, certPem, keyPem string, listenPort uint16) {
	binary := ensureNaiveServer(t)

	configTemplate, err := os.ReadFile("config/sing-box-quic.json")
	require.NoError(t, err)

	certPem = filepath.ToSlash(certPem)
	keyPem = filepath.ToSlash(keyPem)

	config := strings.ReplaceAll(string(configTemplate), "/cert.pem", certPem)
	config = strings.ReplaceAll(config, "/key.pem", keyPem)
	config = strings.Replace(config, `"listen_port": 10002`, fmt.Sprintf(`"listen_port": %d`, listenPort), 1)

	configPath := filepath.Join(t.TempDir(), "sing-box-quic.json")
	err = os.WriteFile(configPath, []byte(config), 0o644)
	require.NoError(t, err)

	startNaiveServerWithConfig(t, binary, configPath, listenPort, "udp")
}

func startNaiveQUICServerWithECH(t *testing.T, certPath, keyPath, echKey string, listenPort uint16) {
	binary := ensureNaiveServer(t)

	echKeyPath := filepath.Join(t.TempDir(), "ech_key.pem")
	err := os.WriteFile(echKeyPath, []byte(echKey), 0o644)
	require.NoError(t, err)

	certPath = filepath.ToSlash(certPath)
	keyPath = filepath.ToSlash(keyPath)
	echKeyPath = filepath.ToSlash(echKeyPath)

	config := fmt.Sprintf(`{
  "inbounds": [
    {
      "type": "naive",
      "listen": "::",
      "listen_port": %d,
      "network": "udp",
      "users": [
        {
          "username": "test",
          "password": "test"
        }
      ],
      "tls": {
        "enabled": true,
        "certificate_path": "%s",
        "key_path": "%s",
        "ech": {
          "enabled": true,
          "key_path": "%s"
        }
      }
    }
  ]
}`, listenPort, certPath, keyPath, echKeyPath)

	configPath := filepath.Join(t.TempDir(), "sing-box-quic-ech.json")
	err = os.WriteFile(configPath, []byte(config), 0o644)
	require.NoError(t, err)

	startNaiveServerWithConfig(t, binary, configPath, listenPort, "udp")
}

// TestNaiveQUIC verifies NaiveClient connectivity with QUIC protocol.
func TestNaiveQUIC(t *testing.T) {
	naiveQUICServerPort := reserveUDPPort(t)
	caPem, certPem, keyPem := generateCertificate(t, "example.org")
	caPemContent, err := os.ReadFile(caPem)
	require.NoError(t, err)

	startNaiveQUICServer(t, certPem, keyPem, naiveQUICServerPort)

	client, err := cronet.NewNaiveClient(cronet.NaiveClientOptions{
		ServerAddress:           M.ParseSocksaddrHostPort("127.0.0.1", naiveQUICServerPort),
		ServerName:              "example.org",
		Username:                "test",
		Password:                "test",
		TrustedRootCertificates: string(caPemContent),
		DNSResolver:             localhostDNSResolverWithHTTPSResponse(t, naiveQUICServerPort, []string{"h3"}),

		QUIC:                    true,
	})
	require.NoError(t, err)
	require.NoError(t, client.Start())
	t.Cleanup(func() { client.Close() })
	_ = startNetLogForTest(t, client, "quic_basic_netlog.json", false)

	echoPort := reserveTCPPort(t)
	startEchoServer(t, echoPort)

	conn, err := client.DialEarly(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", echoPort))
	require.NoError(t, err)
	defer conn.Close()

	testData := []byte("Hello, NaiveProxy QUIC!")
	_, err = conn.Write(testData)
	require.NoError(t, err)

	buf := make([]byte, len(testData))
	_, err = io.ReadFull(conn, buf)
	require.NoError(t, err)
	require.Equal(t, testData, buf)
}

// TestNaiveQUICLargeTransfer tests data integrity with large transfers over QUIC.
func TestNaiveQUICLargeTransfer(t *testing.T) {
	naiveQUICServerPort := reserveUDPPort(t)
	caPem, certPem, keyPem := generateCertificate(t, "example.org")
	caPemContent, err := os.ReadFile(caPem)
	require.NoError(t, err)

	startNaiveQUICServer(t, certPem, keyPem, naiveQUICServerPort)

	client, err := cronet.NewNaiveClient(cronet.NaiveClientOptions{
		ServerAddress:           M.ParseSocksaddrHostPort("127.0.0.1", naiveQUICServerPort),
		ServerName:              "example.org",
		Username:                "test",
		Password:                "test",
		TrustedRootCertificates: string(caPemContent),
		DNSResolver:             localhostDNSResolverWithHTTPSResponse(t, naiveQUICServerPort, []string{"h3"}),

		QUIC:                    true,
	})
	require.NoError(t, err)
	require.NoError(t, client.Start())
	t.Cleanup(func() { client.Close() })
	_ = startNetLogForTest(t, client, "quic_large_transfer_netlog.json", false)

	echoPort := reserveTCPPort(t)
	startEchoServer(t, echoPort)

	conn, err := client.DialEarly(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", echoPort))
	require.NoError(t, err)
	defer conn.Close()

	// Generate 256KB of test data
	const dataSize = 256 * 1024
	testData := make([]byte, dataSize)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

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
	require.Equal(t, testData, receivedData, "data mismatch in large transfer over QUIC")
}

func TestNaiveQUICDomainNon443DoesNotIssueHTTPSDNSQueryByDefault(t *testing.T) {
	naiveQUICServerPort := reserveUDPPort(t)
	caPem, certPem, keyPem := generateCertificate(t, "example.org")
	caPemContent, err := os.ReadFile(caPem)
	require.NoError(t, err)

	startNaiveQUICServer(t, certPem, keyPem, naiveQUICServerPort)

	queryObservation := &dnsQueryObservation{}
	dnsResolver := makeQUICDomainResolver(queryObservation, 0, mDNS.RcodeNameError)

	client, err := cronet.NewNaiveClient(cronet.NaiveClientOptions{
		ServerAddress:           M.ParseSocksaddrHostPort("example.org", naiveQUICServerPort),
		ServerName:              "example.org",
		Username:                "test",
		Password:                "test",
		TrustedRootCertificates: string(caPemContent),
		DNSResolver:             dnsResolver,

		QUIC:                    true,
	})
	require.NoError(t, err)
	require.NoError(t, client.Start())
	t.Cleanup(func() { client.Close() })
	_ = startNetLogForTest(t, client, "quic_non443_default_netlog.json", false)

	echoPort := reserveTCPPort(t)
	startEchoServer(t, echoPort)

	conn, err := client.DialEarly(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", echoPort))
	require.NoError(t, err)
	defer conn.Close()

	testData := []byte("quic non-443 dns query test")
	_, err = conn.Write(testData)
	require.NoError(t, err)

	buffer := make([]byte, len(testData))
	_, err = io.ReadFull(conn, buffer)
	require.NoError(t, err)
	require.Equal(t, testData, buffer)

	require.Greater(t, queryObservation.aQueryCount.Load(), int64(0))
	require.Equal(
		t,
		int64(0),
		queryObservation.httpsQueryCount.Load(),
		"unexpected HTTPS DNS query in default QUIC mode: %v",
		queryObservation.queryNames(),
	)
}

func TestNaiveQUICDomainNon443ECHHTTPSDNSDelayAffectsHandshake(t *testing.T) {
	naiveQUICServerPort := reserveUDPPort(t)
	caPem, certPem, keyPem := generateCertificate(t, "example.org")
	caPemContent, err := os.ReadFile(caPem)
	require.NoError(t, err)

	startNaiveQUICServer(t, certPem, keyPem, naiveQUICServerPort)

	queryObservation := &dnsQueryObservation{}
	dnsResolver := makeQUICDomainResolver(queryObservation, 2*time.Second, mDNS.RcodeNameError)

	client, err := cronet.NewNaiveClient(cronet.NaiveClientOptions{
		ServerAddress:           M.ParseSocksaddrHostPort("example.org", naiveQUICServerPort),
		ServerName:              "example.org",
		Username:                "test",
		Password:                "test",
		TrustedRootCertificates: string(caPemContent),
		DNSResolver:             dnsResolver,
		ECHEnabled:              true,
		QUIC:                    true,

	})
	require.NoError(t, err)
	require.NoError(t, client.Start())
	t.Cleanup(func() { client.Close() })
	_ = startNetLogForTest(t, client, "quic_non443_ech_dns_delay_netlog.json", false)

	echoPort := reserveTCPPort(t)
	startEchoServer(t, echoPort)

	conn, err := client.DialEarly(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", echoPort))
	require.NoError(t, err)
	defer conn.Close()

	testData := []byte("quic non-443 dns delay test")
	startTime := time.Now()
	_, err = conn.Write(testData)
	require.NoError(t, err)
	handshakeDuration := time.Since(startTime)

	buffer := make([]byte, len(testData))
	_, err = io.ReadFull(conn, buffer)
	require.NoError(t, err)
	require.Equal(t, testData, buffer)

	require.GreaterOrEqual(t, handshakeDuration, 1500*time.Millisecond)
	require.Greater(t, queryObservation.httpsQueryCount.Load(), int64(0))
	require.True(
		t,
		queryObservation.hasHTTPSPortQuery(fmt.Sprintf("_%d._https.example.org", naiveQUICServerPort)),
		"expected HTTPS query for _%d._https.example.org, got %v",
		naiveQUICServerPort,
		queryObservation.queryNames(),
	)
}

func TestNaiveQUICDomainNon443ECHFixedConfigDisablesHTTPSLookup(t *testing.T) {
	naiveQUICServerPort := reserveUDPPort(t)
	echConfigPEM, echKeyPEM, err := echKeygenDefault("not.example.org")
	require.NoError(t, err)

	echConfigBlock, _ := pem.Decode([]byte(echConfigPEM))
	require.NotNil(t, echConfigBlock)

	caPem, certPem, keyPem := generateCertificate(t, "example.org")
	caPemContent, err := os.ReadFile(caPem)
	require.NoError(t, err)

	startNaiveQUICServerWithECH(t, certPem, keyPem, echKeyPEM, naiveQUICServerPort)

	queryObservation := &dnsQueryObservation{}
	dnsResolver := makeQUICDomainResolver(queryObservation, 2*time.Second, mDNS.RcodeNameError)

	client, err := cronet.NewNaiveClient(cronet.NaiveClientOptions{
		ServerAddress:           M.ParseSocksaddrHostPort("example.org", naiveQUICServerPort),
		ServerName:              "example.org",
		Username:                "test",
		Password:                "test",
		TrustedRootCertificates: string(caPemContent),
		DNSResolver:             dnsResolver,
		ECHEnabled:              true,
		ECHConfigList:           echConfigBlock.Bytes,
		QUIC:                    true,

	})
	require.NoError(t, err)
	require.NoError(t, client.Start())
	t.Cleanup(func() { client.Close() })

	netLogPath := startNetLogForTest(t, client, "quic_ech_fixed_config_netlog.json", true)

	echoPort := reserveTCPPort(t)
	startEchoServer(t, echoPort)

	conn, err := client.DialEarly(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", echoPort))
	require.NoError(t, err)
	defer conn.Close()

	startTime := time.Now()
	testData := []byte("quic non-443 fixed ech config")
	_, err = conn.Write(testData)
	require.NoError(t, err)
	handshakeDuration := time.Since(startTime)

	buffer := make([]byte, len(testData))
	_, err = io.ReadFull(conn, buffer)
	require.NoError(t, err)
	require.Equal(t, testData, buffer)

	client.Engine().StopNetLog()

	logContent, err := os.ReadFile(netLogPath)
	require.NoError(t, err)
	logString := string(logContent)

	require.Less(t, handshakeDuration, 3500*time.Millisecond)
	require.Equal(t, int64(0), queryObservation.httpsQueryCount.Load(), "unexpected resolver HTTPS query: %v", queryObservation.queryNames())
	require.Contains(t, logString, fmt.Sprintf("_%d._https.example.org", naiveQUICServerPort))
}

func TestNaiveQUICFixedIPSkipsServerDNSQueries(t *testing.T) {
	naiveQUICServerPort := reserveUDPPort(t)
	caPem, certPem, keyPem := generateCertificate(t, "example.org")
	caPemContent, err := os.ReadFile(caPem)
	require.NoError(t, err)

	startNaiveQUICServer(t, certPem, keyPem, naiveQUICServerPort)

	queryObservation := &dnsQueryObservation{}
	dnsResolver := makeQUICDomainResolver(queryObservation, 0, mDNS.RcodeNameError)

	client, err := cronet.NewNaiveClient(cronet.NaiveClientOptions{
		ServerAddress:           M.ParseSocksaddrHostPort("127.0.0.1", naiveQUICServerPort),
		ServerName:              "example.org",
		Username:                "test",
		Password:                "test",
		TrustedRootCertificates: string(caPemContent),
		DNSResolver:             dnsResolver,
		QUIC:                    true,

	})
	require.NoError(t, err)
	require.NoError(t, client.Start())
	t.Cleanup(func() { client.Close() })
	_ = startNetLogForTest(t, client, "quic_fixed_ip_netlog.json", false)

	echoPort := reserveTCPPort(t)
	startEchoServer(t, echoPort)

	testData := []byte("quic fixed ip dns test")
	target := M.ParseSocksaddrHostPort("127.0.0.1", echoPort)

	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		conn, err := client.DialEarly(context.Background(), target)
		if err != nil {
			lastErr = err
		} else {
			_, err = conn.Write(testData)
			if err == nil {
				buffer := make([]byte, len(testData))
				_, err = io.ReadFull(conn, buffer)
				if err == nil && !bytes.Equal(buffer, testData) {
					err = fmt.Errorf("unexpected echo payload")
				}
			}
			_ = conn.Close()
			lastErr = err
		}

		if lastErr == nil {
			break
		}
		if attempt < maxAttempts && isRetryableQUICProtocolError(lastErr) {
			t.Logf("retrying after transient QUIC error (%d/%d): %v", attempt, maxAttempts, lastErr)
			time.Sleep(200 * time.Millisecond)
			continue
		}
		require.NoError(t, lastErr)
	}
	require.NoError(t, lastErr)

	totalQueryCount := queryObservation.aQueryCount.Load() +
		queryObservation.aaaaQueryCount.Load() +
		queryObservation.httpsQueryCount.Load()
	require.Equal(t, int64(0), totalQueryCount, "expected zero DNS queries, got %v", queryObservation.queryNames())
}

func isRetryableQUICProtocolError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "quic protocol error")
}

type dnsQueryObservation struct {
	aQueryCount     atomic.Int64
	aaaaQueryCount  atomic.Int64
	httpsQueryCount atomic.Int64
	namesMutex      sync.Mutex
	names           []string
}

func (o *dnsQueryObservation) record(question mDNS.Question) {
	name := strings.TrimSuffix(strings.ToLower(question.Name), ".")

	o.namesMutex.Lock()
	o.names = append(o.names, name)
	o.namesMutex.Unlock()

	switch question.Qtype {
	case mDNS.TypeA:
		o.aQueryCount.Add(1)
	case mDNS.TypeAAAA:
		o.aaaaQueryCount.Add(1)
	case mDNS.TypeHTTPS:
		o.httpsQueryCount.Add(1)
	}
}

func (o *dnsQueryObservation) queryNames() []string {
	o.namesMutex.Lock()
	defer o.namesMutex.Unlock()

	names := make([]string, len(o.names))
	copy(names, o.names)
	return names
}

func (o *dnsQueryObservation) hasHTTPSPortQuery(queryPrefix string) bool {
	queryPrefix = strings.ToLower(queryPrefix)
	for _, queryName := range o.queryNames() {
		if strings.HasPrefix(queryName, queryPrefix) {
			return true
		}
	}
	return false
}

func makeQUICDomainResolver(
	observation *dnsQueryObservation,
	httpsResponseDelay time.Duration,
	httpsResponseCode int,
) cronet.DNSResolverFunc {
	return func(ctx context.Context, request *mDNS.Msg) *mDNS.Msg {
		response := new(mDNS.Msg)
		response.SetReply(request)

		for _, question := range request.Question {
			observation.record(question)

			switch question.Qtype {
			case mDNS.TypeA:
				response.Answer = append(response.Answer, &mDNS.A{
					Hdr: mDNS.RR_Header{
						Name:   question.Name,
						Rrtype: mDNS.TypeA,
						Class:  mDNS.ClassINET,
						Ttl:    300,
					},
					A: net.ParseIP("127.0.0.1").To4(),
				})
			case mDNS.TypeAAAA:
			case mDNS.TypeHTTPS:
				if httpsResponseDelay > 0 {
					select {
					case <-time.After(httpsResponseDelay):
					case <-ctx.Done():
						return response
					}
				}
				response.Rcode = httpsResponseCode
			}
		}
		return response
	}
}
