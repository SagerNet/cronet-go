package test

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cronet "github.com/sagernet/cronet-go"
	M "github.com/sagernet/sing/common/metadata"

	mDNS "github.com/miekg/dns"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/cryptobyte"
)

// TestNaiveECH verifies that ECH (Encrypted Client Hello) works with NaiveClient.
// It starts a sing-box naive server with ECH enabled, then connects using
// NaiveClient with ECH config injected via DNS HTTPS records.
func TestNaiveECH(t *testing.T) {
	echServerPort := reserveTCPPort(t)
	echEchoPort := reserveTCPPort(t)

	// Generate ECH config and key
	echConfigPEM, echKeyPEM, err := echKeygenDefault("not.example.org")
	require.NoError(t, err)

	// Decode ECH config from PEM to wire format
	echConfigBlock, _ := pem.Decode([]byte(echConfigPEM))
	require.NotNil(t, echConfigBlock, "failed to decode ECH config PEM")
	echConfigWire := echConfigBlock.Bytes

	// Generate certificates
	caPEM, certPath, keyPath := generateCertificate(t, "example.org")
	caPEMContent, err := os.ReadFile(caPEM)
	require.NoError(t, err)

	// Start naive server with ECH
	startNaiveServerWithECH(t, certPath, keyPath, echKeyPEM, echServerPort)

	// Start echo server
	startEchoServer(t, echEchoPort)

	// Create DNS resolver that returns localhost and handles HTTPS queries
	dnsResolver := func(ctx context.Context, request *mDNS.Msg) *mDNS.Msg {
		t.Logf("DNS resolver called: %v", request.Question)
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
			case mDNS.TypeHTTPS:
				t.Logf("HTTPS query for %s - will be handled by ECH wrapper", question.Name)
				// The ECH config will be injected by wrapDNSResolverWithECH
			}
		}
		return response
	}

	// Create NaiveClient with ECH config
	client, err := cronet.NewNaiveClient(cronet.NaiveClientOptions{
		ServerAddress:           M.ParseSocksaddrHostPort("127.0.0.1", echServerPort),
		ServerName:              "example.org",
		Username:                "test",
		Password:                "test",
		TrustedRootCertificates: string(caPEMContent),
		DNSResolver:             dnsResolver,
		ECHEnabled:              true,
		ECHConfigList:           echConfigWire,
	})
	require.NoError(t, err)
	require.NoError(t, client.Start())
	t.Cleanup(func() { client.Close() })

	// Start NetLog to capture TLS handshake details
	netLogPath := startNetLogForTest(t, client, "ech_netlog.json", true)
	defer client.Engine().StopNetLog()

	// Make a connection
	conn, err := client.DialEarly(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", echEchoPort))
	require.NoError(t, err)
	defer conn.Close()

	// Test data transfer
	testData := []byte("ECH test data!")
	_, err = conn.Write(testData)
	require.NoError(t, err)

	buf := make([]byte, len(testData))
	_, err = io.ReadFull(conn, buf)
	require.NoError(t, err)
	require.Equal(t, testData, buf)

	// Stop NetLog and verify ECH was used
	client.Engine().StopNetLog()

	logContent, err := os.ReadFile(netLogPath)
	require.NoError(t, err)
	logStr := string(logContent)

	// Check that ECH was accepted in TLS handshake
	// The NetLog should contain "encrypted_client_hello":true when ECH is used
	require.True(t, strings.Contains(logStr, `"encrypted_client_hello":true`),
		"ECH should be accepted in TLS handshake. NetLog saved to: %s", netLogPath)
}

// startNaiveServerWithECH starts a sing-box naive server with ECH enabled.
func startNaiveServerWithECH(t *testing.T, certPath, keyPath, echKey string, port uint16) {
	binary := ensureNaiveServer(t)

	// Create ECH config directory and write key file
	echKeyPath := filepath.Join(t.TempDir(), "ech_key.pem")
	err := os.WriteFile(echKeyPath, []byte(echKey), 0o644)
	require.NoError(t, err)

	// Use forward slashes for JSON compatibility
	certPath = filepath.ToSlash(certPath)
	keyPath = filepath.ToSlash(keyPath)
	echKeyPath = filepath.ToSlash(echKeyPath)

	// Create config with ECH enabled
	config := fmt.Sprintf(`{
  "inbounds": [
    {
      "type": "naive",
      "listen": "::",
      "listen_port": %d,
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
}`, port, certPath, keyPath, echKeyPath)

	configPath := filepath.Join(t.TempDir(), "sing-box-ech.json")
	err = os.WriteFile(configPath, []byte(config), 0o644)
	require.NoError(t, err)

	startNaiveServerWithConfig(t, binary, configPath, port, "tcp")
}

// echKeygenDefault generates ECH config and key in PEM format.
// Ported from sing-box/common/tls/ech_shared.go.
func echKeygenDefault(publicName string) (configPEM string, keyPEM string, err error) {
	echKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return
	}
	echConfig, err := marshalECHConfig(0, echKey.PublicKey().Bytes(), publicName, 0)
	if err != nil {
		return
	}
	configBuilder := cryptobyte.NewBuilder(nil)
	configBuilder.AddUint16LengthPrefixed(func(builder *cryptobyte.Builder) {
		builder.AddBytes(echConfig)
	})
	configBytes, err := configBuilder.Bytes()
	if err != nil {
		return
	}
	keyBuilder := cryptobyte.NewBuilder(nil)
	keyBuilder.AddUint16LengthPrefixed(func(builder *cryptobyte.Builder) {
		builder.AddBytes(echKey.Bytes())
	})
	keyBuilder.AddUint16LengthPrefixed(func(builder *cryptobyte.Builder) {
		builder.AddBytes(echConfig)
	})
	keyBytes, err := keyBuilder.Bytes()
	if err != nil {
		return
	}
	configPEM = string(pem.EncodeToMemory(&pem.Block{Type: "ECH CONFIGS", Bytes: configBytes}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "ECH KEYS", Bytes: keyBytes}))
	return
}

// marshalECHConfig creates an ECH config in wire format.
// Ported from sing-box/common/tls/ech_shared.go.
func marshalECHConfig(id uint8, pubKey []byte, publicName string, maxNameLen uint8) ([]byte, error) {
	const extensionEncryptedClientHello = 0xfe0d
	const dhkemX25519HKDFSHA256 = 0x0020
	const kdfHKDFSHA256 = 0x0001
	builder := cryptobyte.NewBuilder(nil)
	builder.AddUint16(extensionEncryptedClientHello)
	builder.AddUint16LengthPrefixed(func(builder *cryptobyte.Builder) {
		builder.AddUint8(id)

		builder.AddUint16(dhkemX25519HKDFSHA256) // The only DHKEM we support
		builder.AddUint16LengthPrefixed(func(builder *cryptobyte.Builder) {
			builder.AddBytes(pubKey)
		})
		builder.AddUint16LengthPrefixed(func(builder *cryptobyte.Builder) {
			const (
				aeadAES128GCM      = 0x0001
				aeadAES256GCM      = 0x0002
				aeadChaCha20Poly05 = 0x0003
			)
			for _, aeadID := range []uint16{aeadAES128GCM, aeadAES256GCM, aeadChaCha20Poly05} {
				builder.AddUint16(kdfHKDFSHA256) // The only KDF we support
				builder.AddUint16(aeadID)
			}
		})
		builder.AddUint8(maxNameLen)
		builder.AddUint8LengthPrefixed(func(builder *cryptobyte.Builder) {
			builder.AddBytes([]byte(publicName))
		})
		builder.AddUint16(0) // extensions
	})
	return builder.Bytes()
}
