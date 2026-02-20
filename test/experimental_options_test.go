package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	cronet "github.com/sagernet/cronet-go"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/stretchr/testify/require"
)

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
