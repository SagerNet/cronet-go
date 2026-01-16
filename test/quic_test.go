package test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cronet "github.com/sagernet/cronet-go"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/stretchr/testify/require"
)

const naiveQUICServerPort = 10002

func startNaiveQUICServer(t *testing.T, certPem, keyPem string) {
	binary := ensureNaiveServer(t)

	configTemplate, err := os.ReadFile("config/sing-box-quic.json")
	require.NoError(t, err)

	certPem = filepath.ToSlash(certPem)
	keyPem = filepath.ToSlash(keyPem)

	config := strings.ReplaceAll(string(configTemplate), "/cert.pem", certPem)
	config = strings.ReplaceAll(config, "/key.pem", keyPem)

	configPath := filepath.Join(t.TempDir(), "sing-box-quic.json")
	err = os.WriteFile(configPath, []byte(config), 0o644)
	require.NoError(t, err)

	startNaiveServerWithConfig(t, binary, configPath)
}

// TestNaiveQUIC verifies NaiveClient connectivity with QUIC protocol.
func TestNaiveQUIC(t *testing.T) {
	caPem, certPem, keyPem := generateCertificate(t, "example.org")
	caPemContent, err := os.ReadFile(caPem)
	require.NoError(t, err)

	startNaiveQUICServer(t, certPem, keyPem)
	time.Sleep(time.Second)

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

	startEchoServer(t, 18200)

	conn, err := client.DialEarly(M.ParseSocksaddrHostPort("127.0.0.1", 18200))
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
	caPem, certPem, keyPem := generateCertificate(t, "example.org")
	caPemContent, err := os.ReadFile(caPem)
	require.NoError(t, err)

	startNaiveQUICServer(t, certPem, keyPem)
	time.Sleep(time.Second)

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

	startEchoServer(t, 18201)

	conn, err := client.DialEarly(M.ParseSocksaddrHostPort("127.0.0.1", 18201))
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
