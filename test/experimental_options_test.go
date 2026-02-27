package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	cronet "github.com/sagernet/cronet-go"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/stretchr/testify/require"
)

type netLogData struct {
	Constants struct {
		LogEventTypes map[string]int `json:"logEventTypes"`
	} `json:"constants"`
	Events []struct {
		Type   int             `json:"type"`
		Params json.RawMessage `json:"params"`
	} `json:"events"`
}

func parseQUICTransportParametersSent(t *testing.T, logContent []byte) string {
	t.Helper()
	var netLog netLogData
	err := json.Unmarshal(logContent, &netLog)
	require.NoError(t, err)

	eventType, ok := netLog.Constants.LogEventTypes["QUIC_SESSION_TRANSPORT_PARAMETERS_SENT"]
	require.True(t, ok, "QUIC_SESSION_TRANSPORT_PARAMETERS_SENT not in netlog constants")

	for _, event := range netLog.Events {
		if event.Type == eventType {
			var params struct {
				QuicTransportParameters string `json:"quic_transport_parameters"`
			}
			err := json.Unmarshal(event.Params, &params)
			require.NoError(t, err)
			if params.QuicTransportParameters != "" {
				return params.QuicTransportParameters
			}
		}
	}
	t.Fatal("QUIC_SESSION_TRANSPORT_PARAMETERS_SENT event not found in netlog")
	return ""
}

func TestHTTP2Options(t *testing.T) {
	const (
		sessionMaxReceiveWindowSize   = 134217728
		streamInitialWindowSize       = 67108864
		defaultHTTP2InitialWindowSize = 65535
	)

	env := setupTestEnv(t)
	client := env.newNaiveClient(t, cronet.NaiveClientOptions{
		DNSResolver: localhostDNSResolver(t),
	})

	startEchoServer(t, 18100)

	netLogPath := filepath.Join(t.TempDir(), "http2_options_netlog.json")
	require.True(t, client.Engine().StartNetLogToFile(netLogPath, true),
		"Failed to start NetLog")

	conn, err := client.DialEarly(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", 18100))
	require.NoError(t, err)

	testData := []byte("hello")
	_, err = conn.Write(testData)
	require.NoError(t, err)

	buffer := make([]byte, len(testData))
	_, err = io.ReadFull(conn, buffer)
	require.NoError(t, err)
	require.Equal(t, testData, buffer)

	conn.Close()
	client.Engine().StopNetLog()

	logContent, err := os.ReadFile(netLogPath)
	require.NoError(t, err)
	logString := string(logContent)

	var netLog struct {
		Constants struct {
			LogEventTypes map[string]int `json:"logEventTypes"`
		} `json:"constants"`
		Events []struct {
			Type   int             `json:"type"`
			Params json.RawMessage `json:"params"`
		} `json:"events"`
	}
	err = json.Unmarshal(logContent, &netLog)
	require.NoError(t, err)

	// HTTP2_SESSION_SEND_SETTINGS should contain SETTINGS_INITIAL_WINDOW_SIZE =
	// 67108864 (64 MB, naive default).
	sendSettingsType, ok := netLog.Constants.LogEventTypes["HTTP2_SESSION_SEND_SETTINGS"]
	require.True(t, ok, "expected HTTP2_SESSION_SEND_SETTINGS in NetLog constants")
	sendSettingsFound := false
	for _, event := range netLog.Events {
		if event.Type == sendSettingsType {
			sendSettingsFound = true
			break
		}
	}
	require.True(t, sendSettingsFound, "expected HTTP2_SESSION_SEND_SETTINGS event in netlog")
	require.Contains(t, logString,
		fmt.Sprintf("SETTINGS_INITIAL_WINDOW_SIZE) value:%d]", streamInitialWindowSize),
		"expected SETTINGS_INITIAL_WINDOW_SIZE = 64 MB in HTTP/2 SETTINGS")

	// HTTP2_SESSION_SEND_WINDOW_UPDATE with stream_id 0 should carry
	// delta = session_max_recv_window_size - default_initial_window_size.
	sendWindowUpdateType, ok := netLog.Constants.LogEventTypes["HTTP2_SESSION_SEND_WINDOW_UPDATE"]
	require.True(t, ok, "expected HTTP2_SESSION_SEND_WINDOW_UPDATE in NetLog constants")
	expectedSessionWindowDelta := sessionMaxReceiveWindowSize - defaultHTTP2InitialWindowSize
	sessionWindowUpdateFound := false
	for _, event := range netLog.Events {
		if event.Type != sendWindowUpdateType {
			continue
		}

		var params struct {
			StreamID *int `json:"stream_id"`
			Delta    *int `json:"delta"`
		}
		err = json.Unmarshal(event.Params, &params)
		require.NoError(t, err)

		if params.StreamID != nil &&
			params.Delta != nil &&
			*params.StreamID == 0 &&
			*params.Delta == expectedSessionWindowDelta {
			sessionWindowUpdateFound = true
			break
		}
	}
	require.True(t, sessionWindowUpdateFound,
		"expected session WINDOW_UPDATE event with stream_id=0 and delta=%d", expectedSessionWindowDelta)
}

func TestSocketPoolOptions(t *testing.T) {
	params := cronet.NewEngineParams()
	defer params.Destroy()

	err := params.SetSocketPoolOptions(2048, 2048, 2040)
	require.NoError(t, err)

	options := params.ExperimentalOptions()
	require.Contains(t, options, `"SocketPoolOptions"`,
		"expected SocketPoolOptions key, got: %s", options)
	require.Contains(t, options, `"max_sockets_per_pool":2048`,
		"expected max_sockets_per_pool in options, got: %s", options)
	require.Contains(t, options, `"max_sockets_per_proxy_chain":2048`,
		"expected max_sockets_per_proxy_chain in options, got: %s", options)
	require.Contains(t, options, `"max_sockets_per_group":2040`,
		"expected max_sockets_per_group in options, got: %s", options)
}

func TestQUICReceiveWindowCustomValues(t *testing.T) {
	caPem, certPem, keyPem := generateCertificate(t, "example.org")
	caPemContent, err := os.ReadFile(caPem)
	require.NoError(t, err)

	startNaiveQUICServer(t, certPem, keyPem)
	time.Sleep(time.Second)

	const (
		customStreamWindow  = 4 * 1024 * 1024
		customSessionWindow = 10 * 1024 * 1024
	)

	client, err := cronet.NewNaiveClient(cronet.NaiveClientOptions{
		ServerAddress:           M.ParseSocksaddrHostPort("127.0.0.1", naiveQUICServerPort),
		ServerName:              "example.org",
		Username:                "test",
		Password:                "test",
		TrustedRootCertificates: string(caPemContent),
		DNSResolver:             localhostDNSResolverWithHTTPSResponse(t, naiveQUICServerPort, []string{"h3"}),
		QUIC:                    true,
		StreamReceiveWindow:     customStreamWindow,
		SessionReceiveWindow:    customSessionWindow,
	})
	require.NoError(t, err)
	require.NoError(t, client.Start())
	t.Cleanup(func() { client.Close() })

	startEchoServer(t, 18300)

	netLogPath := filepath.Join(t.TempDir(), "quic_recv_window_custom.json")
	require.True(t, client.Engine().StartNetLogToFile(netLogPath, true))

	conn, err := client.DialEarly(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", 18300))
	require.NoError(t, err)

	testData := []byte("quic custom window test")
	_, err = conn.Write(testData)
	require.NoError(t, err)
	buf := make([]byte, len(testData))
	_, err = io.ReadFull(conn, buf)
	require.NoError(t, err)
	require.Equal(t, testData, buf)

	conn.Close()
	client.Engine().StopNetLog()

	logContent, err := os.ReadFile(netLogPath)
	require.NoError(t, err)

	tp := parseQUICTransportParametersSent(t, logContent)
	require.Contains(t, tp, fmt.Sprintf("initial_max_data %d", customSessionWindow),
		"expected session receive window in transport parameters")
	require.Contains(t, tp, fmt.Sprintf("initial_max_stream_data_bidi_local %d", customStreamWindow),
		"expected stream receive window in transport parameters")
}

func TestQUICReceiveWindowDefaults(t *testing.T) {
	caPem, certPem, keyPem := generateCertificate(t, "example.org")
	caPemContent, err := os.ReadFile(caPem)
	require.NoError(t, err)

	startNaiveQUICServer(t, certPem, keyPem)
	time.Sleep(time.Second)

	const (
		defaultStreamWindow  = 8 * 1024 * 1024
		defaultSessionWindow = 20 * 1024 * 1024
	)

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

	startEchoServer(t, 18301)

	netLogPath := filepath.Join(t.TempDir(), "quic_recv_window_defaults.json")
	require.True(t, client.Engine().StartNetLogToFile(netLogPath, true))

	conn, err := client.DialEarly(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", 18301))
	require.NoError(t, err)

	testData := []byte("quic default window test")
	_, err = conn.Write(testData)
	require.NoError(t, err)
	buf := make([]byte, len(testData))
	_, err = io.ReadFull(conn, buf)
	require.NoError(t, err)
	require.Equal(t, testData, buf)

	conn.Close()
	client.Engine().StopNetLog()

	logContent, err := os.ReadFile(netLogPath)
	require.NoError(t, err)

	tp := parseQUICTransportParametersSent(t, logContent)
	require.Contains(t, tp, fmt.Sprintf("initial_max_data %d", defaultSessionWindow),
		"expected default session receive window in transport parameters")
	require.Contains(t, tp, fmt.Sprintf("initial_max_stream_data_bidi_local %d", defaultStreamWindow),
		"expected default stream receive window in transport parameters")
}

func TestQUICInsecureConcurrencyRejected(t *testing.T) {
	_, err := cronet.NewNaiveClient(cronet.NaiveClientOptions{
		ServerAddress:       M.ParseSocksaddrHostPort("127.0.0.1", 10002),
		DNSResolver:         localhostDNSResolver(t),
		QUIC:                true,
		InsecureConcurrency: 2,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "insecure concurrency is not supported with QUIC")

	client, err := cronet.NewNaiveClient(cronet.NaiveClientOptions{
		ServerAddress:       M.ParseSocksaddrHostPort("127.0.0.1", 10002),
		DNSResolver:         localhostDNSResolver(t),
		QUIC:                true,
		InsecureConcurrency: 1,
	})
	require.NoError(t, err)
	_ = client
}

func TestHTTP2StreamReceiveWindowCustom(t *testing.T) {
	const (
		customStreamWindow            = 32 * 1024 * 1024
		expectedInitialWindowSize     = customStreamWindow / 2
		expectedSessionMaxRecvWindow  = customStreamWindow
		defaultHTTP2InitialWindowSize = 65535
	)

	env := setupTestEnv(t)
	client := env.newNaiveClient(t, cronet.NaiveClientOptions{
		DNSResolver:         localhostDNSResolver(t),
		StreamReceiveWindow: customStreamWindow,
	})

	startEchoServer(t, 18302)

	netLogPath := filepath.Join(t.TempDir(), "http2_custom_stream_window.json")
	require.True(t, client.Engine().StartNetLogToFile(netLogPath, true))

	conn, err := client.DialEarly(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", 18302))
	require.NoError(t, err)

	testData := []byte("http2 custom window")
	_, err = conn.Write(testData)
	require.NoError(t, err)
	buf := make([]byte, len(testData))
	_, err = io.ReadFull(conn, buf)
	require.NoError(t, err)
	require.Equal(t, testData, buf)

	conn.Close()
	client.Engine().StopNetLog()

	logContent, err := os.ReadFile(netLogPath)
	require.NoError(t, err)
	logString := string(logContent)

	require.Contains(t, logString,
		fmt.Sprintf("SETTINGS_INITIAL_WINDOW_SIZE) value:%d]", expectedInitialWindowSize),
		"expected SETTINGS_INITIAL_WINDOW_SIZE = 16 MB")

	var netLog netLogData
	err = json.Unmarshal(logContent, &netLog)
	require.NoError(t, err)

	sendWindowUpdateType, ok := netLog.Constants.LogEventTypes["HTTP2_SESSION_SEND_WINDOW_UPDATE"]
	require.True(t, ok)

	expectedDelta := expectedSessionMaxRecvWindow - defaultHTTP2InitialWindowSize
	found := false
	for _, event := range netLog.Events {
		if event.Type != sendWindowUpdateType {
			continue
		}
		var params struct {
			StreamID *int `json:"stream_id"`
			Delta    *int `json:"delta"`
		}
		err = json.Unmarshal(event.Params, &params)
		require.NoError(t, err)
		if params.StreamID != nil && params.Delta != nil &&
			*params.StreamID == 0 && *params.Delta == expectedDelta {
			found = true
			break
		}
	}
	require.True(t, found,
		"expected session WINDOW_UPDATE with stream_id=0 and delta=%d", expectedDelta)
}
