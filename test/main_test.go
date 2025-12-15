package test

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	cronet "github.com/sagernet/cronet-go"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

const (
	naiveServerPort = 10000
	iperf3Port      = 5201
	forwardPort     = 15201
)

const naiveServerVersion = "1.12.13"

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// testEnv holds shared test environment resources
type testEnv struct {
	caPEM    []byte
	certPath string
	keyPath  string
}

func setupTestEnv(t *testing.T) *testEnv {
	caPem, certPem, keyPem := generateCertificate(t, "example.org")
	caPemContent, err := os.ReadFile(caPem)
	require.NoError(t, err)
	startNaiveServer(t, certPem, keyPem)
	time.Sleep(time.Second)
	return &testEnv{
		caPEM:    caPemContent,
		certPath: certPem,
		keyPath:  keyPem,
	}
}

func (e *testEnv) newNaiveClient(t *testing.T, config cronet.NaiveClientConfig) *cronet.NaiveClient {
	if !config.ServerAddress.IsValid() {
		config.ServerAddress = M.ParseSocksaddrHostPort("127.0.0.1", naiveServerPort)
	}
	if config.ServerName == "" {
		config.ServerName = "example.org"
	}
	if config.Username == "" {
		config.Username = "test"
	}
	if config.Password == "" {
		config.Password = "test"
	}
	if config.TrustedRootCertificates == "" && len(config.TrustedCertificatePublicKeySHA256) == 0 {
		config.TrustedRootCertificates = string(e.caPEM)
	}
	client, err := cronet.NewNaiveClient(config)
	require.NoError(t, err)
	require.NoError(t, client.Start())
	t.Cleanup(func() { client.Close() })
	return client
}

func startEchoServer(t *testing.T, port uint16) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				var transferred int64
				buffer := make([]byte, 32*1024)
				for {
					n, readErr := c.Read(buffer)
					if n > 0 {
						transferred += int64(n)
						_, writeErr := c.Write(buffer[:n])
						if writeErr != nil {
							return
						}
					}
					if readErr != nil {
						if readErr != io.EOF && testing.Verbose() {
							t.Logf("echo server read error: %v", readErr)
						}
						break
					}
				}
				if testing.Verbose() {
					t.Logf("echo server connection closed, bytes=%d", transferred)
				}
			}(conn)
		}
	}()
}

// TestNaiveBasic verifies basic NaiveClient connectivity
func TestNaiveBasic(t *testing.T) {
	env := setupTestEnv(t)
	client := env.newNaiveClient(t, cronet.NaiveClientConfig{})
	startEchoServer(t, 15000)

	conn, err := client.DialEarly(M.ParseSocksaddrHostPort("127.0.0.1", 15000))
	require.NoError(t, err)
	defer conn.Close()

	testData := []byte("Hello, NaiveProxy!")
	_, err = conn.Write(testData)
	require.NoError(t, err)

	buf := make([]byte, len(testData))
	_, err = io.ReadFull(conn, buf)
	require.NoError(t, err)
	require.Equal(t, testData, buf)
}

// TestNaiveIperf3 measures throughput using iperf3
func TestNaiveIperf3(t *testing.T) {
	startIperf3Server(t)
	env := setupTestEnv(t)
	client := env.newNaiveClient(t, cronet.NaiveClientConfig{})

	forwarder := startForwarder(t, forwardPort, client, iperf3Port)
	defer forwarder.Close()

	runIperf3Client(t, forwardPort, 10)
}

// TestNaiveConcurrency tests InsecureConcurrency feature
func TestNaiveConcurrency(t *testing.T) {
	const concurrencyCount = 6

	for i := 0; i < concurrencyCount; i++ {
		startIperf3ServerOnPort(t, uint16(iperf3Port+i))
	}
	env := setupTestEnv(t)
	client := env.newNaiveClient(t, cronet.NaiveClientConfig{InsecureConcurrency: 3})

	var waitGroup sync.WaitGroup
	for i := 0; i < concurrencyCount; i++ {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			serverPort := uint16(iperf3Port + index)
			localPort := uint16(forwardPort + index)
			forwarder := startForwarder(t, localPort, client, serverPort)
			defer forwarder.Close()
			runIperf3Client(t, localPort, 5)
		}(i)
	}
	waitGroup.Wait()
}

// TestNaiveParallel runs multiple iperf3 streams in parallel
func TestNaiveParallel(t *testing.T) {
	startIperf3Server(t)
	env := setupTestEnv(t)
	client := env.newNaiveClient(t, cronet.NaiveClientConfig{})

	forwarder := startForwarder(t, forwardPort, client, iperf3Port)
	defer forwarder.Close()

	cmd := exec.Command("iperf3", "-c", "127.0.0.1", "-p", fmt.Sprint(forwardPort), "-t", "10", "-P", "4")
	output, err := cmd.CombinedOutput()
	t.Log(string(output))
	require.NoError(t, err)
}

// TestNaivePublicKeySHA256 tests certificate public key SHA256 verification
func TestNaivePublicKeySHA256(t *testing.T) {
	env := setupTestEnv(t)

	// Calculate SPKI SHA256
	certPemContent, err := os.ReadFile(env.certPath)
	require.NoError(t, err)
	block, _ := pem.Decode(certPemContent)
	require.NotNil(t, block)
	certificate, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	spkiBytes, err := x509.MarshalPKIXPublicKey(certificate.PublicKey)
	require.NoError(t, err)
	pinHash := sha256.Sum256(spkiBytes)

	client := env.newNaiveClient(t, cronet.NaiveClientConfig{
		TrustedCertificatePublicKeySHA256: [][]byte{pinHash[:]},
	})
	startEchoServer(t, 15001)

	conn, err := client.DialEarly(M.ParseSocksaddrHostPort("127.0.0.1", 15001))
	require.NoError(t, err)
	defer conn.Close()

	testData := []byte("PublicKeySHA256 test!")
	_, err = conn.Write(testData)
	require.NoError(t, err)

	buf := make([]byte, len(testData))
	_, err = io.ReadFull(conn, buf)
	require.NoError(t, err)
	require.Equal(t, testData, buf)
}

// TestNaiveCloseWhileReading tests that Close() works correctly when
// reads are in progress. This verifies the fix for buffer lifetime issues.
func TestNaiveCloseWhileReading(t *testing.T) {
	env := setupTestEnv(t)
	client := env.newNaiveClient(t, cronet.NaiveClientConfig{})
	startEchoServer(t, 16000)

	const iterations = 20

	for i := 0; i < iterations; i++ {
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			conn, err := client.DialContext(ctx, N.NetworkTCP, M.ParseSocksaddrHostPort("127.0.0.1", 16000))
			if err != nil {
				t.Logf("iteration %d: dial failed: %v", i, err)
				return
			}

			// Write some data to trigger response
			testData := make([]byte, 64)
			rand.Read(testData)
			conn.Write(testData)

			readDone := make(chan struct{})
			go func() {
				defer close(readDone)
				buf := make([]byte, 32)
				conn.Read(buf)
			}()

			// Close immediately while read might be in progress
			time.Sleep(time.Millisecond)
			conn.Close()

			select {
			case <-readDone:
			case <-time.After(3 * time.Second):
				t.Fatalf("iteration %d: Read did not return after Close", i)
			}
		}()
	}
}

// Naive server (sing-box) utilities

func ensureNaiveServer(t *testing.T) string {
	binDirectory := "bin"
	err := os.MkdirAll(binDirectory, 0o755)
	require.NoError(t, err)

	binaryName := "sing-box"
	if runtime.GOOS == "windows" {
		binaryName = "sing-box.exe"
	}
	binaryPath := filepath.Join(binDirectory, binaryName)

	// Check if binary already exists
	if _, err := os.Stat(binaryPath); err == nil {
		return binaryPath
	}

	// Download sing-box
	extension := "tar.gz"
	if runtime.GOOS == "windows" {
		extension = "zip"
	}
	downloadURL := fmt.Sprintf(
		"https://github.com/SagerNet/sing-box/releases/download/v%s/sing-box-%s-%s-%s.%s",
		naiveServerVersion, naiveServerVersion, runtime.GOOS, runtime.GOARCH, extension,
	)

	t.Logf("Downloading sing-box from %s", downloadURL)

	// Use custom client and close idle connections after download to avoid goroutine leaks
	httpClient := &http.Client{}
	defer httpClient.CloseIdleConnections()

	response, err := httpClient.Get(downloadURL)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode, "failed to download sing-box: %s", response.Status)

	// Save to temp file
	tempFile, err := os.CreateTemp("", "sing-box-*")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	_, err = io.Copy(tempFile, response.Body)
	require.NoError(t, err)
	tempFile.Close()

	// Extract binary
	if runtime.GOOS == "windows" {
		extractZip(t, tempFile.Name(), binDirectory, binaryName)
	} else {
		extractTarGz(t, tempFile.Name(), binDirectory, binaryName)
	}

	// Make executable on Unix
	if runtime.GOOS != "windows" {
		err = os.Chmod(binaryPath, 0o755)
		require.NoError(t, err)
	}

	return binaryPath
}

func extractTarGz(t *testing.T, archivePath, destDirectory, binaryName string) {
	file, err := os.Open(archivePath)
	require.NoError(t, err)
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	require.NoError(t, err)
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		// Look for the sing-box binary (it's in a subdirectory)
		if header.Typeflag == tar.TypeReg && strings.HasSuffix(header.Name, "/"+binaryName) {
			destPath := filepath.Join(destDirectory, binaryName)
			outFile, err := os.Create(destPath)
			require.NoError(t, err)
			_, err = io.Copy(outFile, tarReader)
			outFile.Close()
			require.NoError(t, err)
			return
		}
	}
	t.Fatalf("binary %s not found in archive", binaryName)
}

func extractZip(t *testing.T, archivePath, destDirectory, binaryName string) {
	zipReader, err := zip.OpenReader(archivePath)
	require.NoError(t, err)
	defer zipReader.Close()

	for _, file := range zipReader.File {
		// Look for the sing-box binary (it's in a subdirectory)
		if strings.HasSuffix(file.Name, "/"+binaryName) || strings.HasSuffix(file.Name, "\\"+binaryName) {
			reader, err := file.Open()
			require.NoError(t, err)

			destPath := filepath.Join(destDirectory, binaryName)
			outFile, err := os.Create(destPath)
			require.NoError(t, err)

			_, err = io.Copy(outFile, reader)
			outFile.Close()
			reader.Close()
			require.NoError(t, err)
			return
		}
	}
	t.Fatalf("binary %s not found in archive", binaryName)
}

func startNaiveServer(t *testing.T, certPem, keyPem string) {
	binary := ensureNaiveServer(t)

	// Read config template and replace certificate paths
	configTemplate, err := os.ReadFile("config/sing-box.json")
	require.NoError(t, err)

	// Use forward slashes for JSON compatibility (works on all platforms)
	certPem = filepath.ToSlash(certPem)
	keyPem = filepath.ToSlash(keyPem)

	config := strings.ReplaceAll(string(configTemplate), "/cert.pem", certPem)
	config = strings.ReplaceAll(config, "/key.pem", keyPem)

	// Write to temp config file
	configPath := filepath.Join(t.TempDir(), "sing-box.json")
	err = os.WriteFile(configPath, []byte(config), 0o644)
	require.NoError(t, err)

	cmd := exec.Command(binary, "run", "-c", configPath)
	if testing.Verbose() {
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
	}
	err = cmd.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})
}

// iperf3 utilities

func startIperf3Server(t *testing.T) {
	startIperf3ServerOnPort(t, iperf3Port)
}

func startIperf3ServerOnPort(t *testing.T, port uint16) {
	cmd := exec.Command("iperf3", "-s", "-p", fmt.Sprint(port))
	if testing.Verbose() {
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
	}
	err := cmd.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})
}

func runIperf3Client(t *testing.T, port uint16, duration int) {
	cmd := exec.Command("iperf3", "-c", "127.0.0.1", "-p", fmt.Sprint(port), "-t", fmt.Sprint(duration))
	output, err := cmd.CombinedOutput()
	t.Log(string(output))
	require.NoError(t, err)
}

// TCP Forwarder

type Forwarder struct {
	listener    net.Listener
	naiveClient *cronet.NaiveClient
	targetPort  uint16
	ctx         context.Context
	cancel      context.CancelFunc
}

func startForwarder(t *testing.T, listenPort uint16, naiveClient *cronet.NaiveClient, targetPort uint16) *Forwarder {
	listener, err := net.Listen("tcp", M.ParseSocksaddrHostPort("127.0.0.1", listenPort).String())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	forwarder := &Forwarder{
		listener:    listener,
		naiveClient: naiveClient,
		targetPort:  targetPort,
		ctx:         ctx,
		cancel:      cancel,
	}

	go forwarder.acceptLoop()
	return forwarder
}

func (f *Forwarder) acceptLoop() {
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			return
		}
		go f.handleConnection(conn)
	}
}

func (f *Forwarder) handleConnection(conn net.Conn) {
	defer conn.Close()

	remoteConn, err := f.naiveClient.DialEarly(M.ParseSocksaddrHostPort("127.0.0.1", f.targetPort))
	if err != nil {
		return
	}
	defer remoteConn.Close()

	bufio.CopyConn(f.ctx, conn, remoteConn)
}

func (f *Forwarder) Close() error {
	f.cancel()
	return f.listener.Close()
}

// Certificate utilities

func generateCertificate(t *testing.T, domain string) (caPem, certPem, keyPem string) {
	tempDir := t.TempDir()

	// Generate CA key
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// CA certificate
	spkiASN1, err := x509.MarshalPKIXPublicKey(caKey.Public())
	require.NoError(t, err)
	var spki struct {
		Algorithm        pkix.AlgorithmIdentifier
		SubjectPublicKey asn1.BitString
	}
	_, err = asn1.Unmarshal(spkiASN1, &spki)
	require.NoError(t, err)
	skid := sha1.Sum(spki.SubjectPublicKey.Bytes)

	caTpl := &x509.Certificate{
		SerialNumber:          randomSerialNumber(t),
		Subject:               pkix.Name{CommonName: "Test CA"},
		SubjectKeyId:          skid[:],
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCert, err := x509.CreateCertificate(rand.Reader, caTpl, caTpl, caKey.Public(), caKey)
	require.NoError(t, err)

	caPem = filepath.Join(tempDir, "ca.pem")
	err = os.WriteFile(caPem, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert}), 0o644)
	require.NoError(t, err)

	// Server key
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Server certificate
	serverTpl := &x509.Certificate{
		SerialNumber: randomSerialNumber(t),
		Subject:      pkix.Name{CommonName: domain},
		DNSNames:     []string{domain},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(0, 1, 0),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	serverCert, err := x509.CreateCertificate(rand.Reader, serverTpl, caTpl, serverKey.Public(), caKey)
	require.NoError(t, err)

	certPem = filepath.Join(tempDir, "cert.pem")
	err = os.WriteFile(certPem, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCert}), 0o644)
	require.NoError(t, err)

	keyPem = filepath.Join(tempDir, "key.pem")
	keyDER, err := x509.MarshalPKCS8PrivateKey(serverKey)
	require.NoError(t, err)
	err = os.WriteFile(keyPem, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o644)
	require.NoError(t, err)

	return
}

func randomSerialNumber(t *testing.T) *big.Int {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	require.NoError(t, err)
	return serialNumber
}
