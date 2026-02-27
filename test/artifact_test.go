package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	cronet "github.com/sagernet/cronet-go"
	"github.com/stretchr/testify/require"
)

const (
	testArtifactDirEnv   = "CRONET_TEST_ARTIFACT_DIR"
	testCaptureNetLogEnv = "CRONET_TEST_CAPTURE_NETLOG"
)

func artifactDir(t *testing.T, category string) string {
	t.Helper()

	baseDir := t.TempDir()
	if configuredDir := strings.TrimSpace(os.Getenv(testArtifactDirEnv)); configuredDir != "" {
		baseDir = filepath.Join(configuredDir, sanitizeArtifactPathSegment(t.Name()))
	}
	if category != "" {
		baseDir = filepath.Join(baseDir, category)
	}

	err := os.MkdirAll(baseDir, 0o755)
	require.NoError(t, err)
	return baseDir
}

func artifactPath(t *testing.T, category, fileName string) string {
	t.Helper()
	return filepath.Join(artifactDir(t, category), fileName)
}

func createArtifactTempFile(t *testing.T, category, pattern string) (*os.File, string) {
	t.Helper()

	file, err := os.CreateTemp(artifactDir(t, category), pattern)
	require.NoError(t, err)
	return file, file.Name()
}

func startNetLogForTest(t *testing.T, client *cronet.NaiveClient, fileName string, required bool) string {
	t.Helper()

	if !required && !shouldCaptureNetLog() {
		return ""
	}

	netLogPath := artifactPath(t, "netlog", fileName)
	started := client.Engine().StartNetLogToFile(netLogPath, true)
	if required {
		require.True(t, started, "failed to start NetLog")
	} else if !started {
		t.Logf("failed to start optional NetLog at %s", netLogPath)
		return ""
	}

	t.Cleanup(func() {
		client.Engine().StopNetLog()
	})
	return netLogPath
}

func shouldCaptureNetLog() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(testCaptureNetLogEnv)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func sanitizeArtifactPathSegment(value string) string {
	sanitized := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			return r
		case r == '-', r == '_', r == '.':
			return r
		default:
			return '_'
		}
	}, value)
	sanitized = strings.Trim(sanitized, "._")
	if sanitized == "" {
		return "test"
	}
	return sanitized
}
