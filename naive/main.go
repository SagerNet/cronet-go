package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/netip"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/sagernet/cronet-go"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/log"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/redir"
	"github.com/sagernet/sing/common/rw"
	"github.com/sagernet/sing/transport/mixed"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var logger = log.NewLogger("naive")

type Config struct {
	Listen              string `json:"listen"`
	Proxy               string `json:"proxy"`
	EnableRedir         bool   `json:"enable-redir"`
	InsecureConcurrency int    `json:"insecure-concurrency"`
	ExtraHeaders        string `json:"extra-headers"`
	HostResolverRules   string `json:"host-resolver-rules"`
	Log                 string `json:"log"`
	NetLog              string `json:"log-net-log"`
	SSLKeyLogFile       string `json:"ssl-key-log-file"`
}

type ExperimentalOptions struct {
	HostResolverRules *HostResolverRules `json:"HostResolverRules,omitempty"`
	SSLKeyLogFile     string             `json:"ssl_key_log_file,omitempty"`
	FeatureList       string             `json:"feature_list,omitempty"`
}

type HostResolverRules struct {
	HostResolverRules string `json:"host_resolver_rules,omitempty"`
}

var config Config

func version() string {
	engine := cronet.NewEngine()
	defer engine.Destroy()
	return "libcronet " + engine.Version()
}

func main() {
	command := &cobra.Command{
		Use:     "naive-go",
		Args:    cobra.MaximumNArgs(1),
		Version: version(),
		Run:     run,
	}
	command.Flags().StringVar(&config.Listen, "listen", "", "<addr:port> set listen address")
	command.Flags().StringVar(&config.Proxy, "proxy", "", "<proto>://[<user>:<pass>@]<hostname>[:<port>] proto: https, quic")
	command.Flags().BoolVar(&config.EnableRedir, "enable-redir", false, "enable redir support (linux only)")
	command.Flags().IntVar(&config.InsecureConcurrency, "insecure-concurrency=", 1, "use N connections, insecure")
	command.Flags().StringVar(&config.ExtraHeaders, "extra-headers", "", "extra headers split by CRLF")
	command.Flags().StringVar(&config.HostResolverRules, "host-resolver-rules", "", "resolver rules")
	command.Flags().StringVar(&config.Log, "log", "disabled", "log to stderr, or file (default disabled)")
	if err := command.Execute(); err != nil {
		logger.Fatalln(err)
	}
}

func run(cmd *cobra.Command, args []string) {
	if len(args) > 0 {
		configContent, err := ioutil.ReadFile(args[0])
		if err != nil {
			logger.Fatal(E.Cause(err, "read config"))
		}
		err = json.Unmarshal(configContent, &config)
		if err != nil {
			logger.Fatal(E.Cause(err, "parse config"))
		}
	}

	if config.Listen == "" {
		_ = cmd.Usage()
		logger.Fatal("missing listen address")
	}

	if config.Log == "disabled" {
		logrus.SetLevel(logrus.InfoLevel)
	} else {
		logrus.SetLevel(logrus.DebugLevel)
		if config.Log != "" {
			logFile, err := os.Create(config.Log)
			if err != nil {
				logger.Fatal(E.Cause(err, "create log"))
			}
			defer logFile.Close()
			logrus.SetOutput(logFile)
		}
	}

	bind, err := netip.ParseAddrPort(config.Listen)
	if err != nil {
		logger.Fatal(E.Cause(err, "parse listen address"))
	}

	var transMode redir.TransproxyMode
	if config.EnableRedir {
		transMode = redir.ModeRedirect
	}

	engine := cronet.NewEngine()
	params := cronet.NewEngineParams()

	if config.HostResolverRules != "" || config.SSLKeyLogFile != "" {
		var experimentalOptions ExperimentalOptions
		if config.HostResolverRules != "" {
			experimentalOptions.HostResolverRules = &HostResolverRules{
				config.HostResolverRules,
			}
		}
		if config.SSLKeyLogFile != "" {
			experimentalOptions.SSLKeyLogFile = config.SSLKeyLogFile
		}
		experimentalOptionsJSON, err := json.Marshal(&experimentalOptions)
		if err != nil {
			logger.Fatal(err)
		}
		params.SetExperimentalOptions(string(experimentalOptionsJSON))
	}

	proxyURL, err := url.Parse(config.Proxy)
	if err != nil {
		logger.Fatal(E.Cause(err, "parse proxy URL"))
	}
	switch proxyURL.Scheme {
	case "https":
		params.SetEnableHTTP2(true)
		params.SetEnableQuic(false)
	case "quic":
		params.SetEnableHTTP2(false)
		params.SetEnableQuic(true)
	default:
		logrus.Fatal("unknown proxy scheme: ", proxyURL.Scheme)
	}
	proxyURL.Scheme = "https"

	var proxyAuthorization string
	if proxyURL.User != nil {
		password, _ := proxyURL.User.Password()
		proxyAuthorization = "Basic " + base64.StdEncoding.EncodeToString([]byte(proxyURL.User.Username()+":"+password))
		proxyURL.User = nil
	}

	engine.StartWithParams(params)
	params.Destroy()

	if config.NetLog != "" {
		engine.StartNetLogToFile(config.NetLog, true)
	}

	listener := &Listener{
		url:           proxyURL.String(),
		authorization: proxyAuthorization,
		engine:        engine.StreamEngine(),
		extraHeaders:  make(map[string]string),
	}

	if config.ExtraHeaders != "" {
		for _, header := range strings.Split(config.ExtraHeaders, "\r\n") {
			hdrArr := strings.SplitN(header, "=", 2)
			listener.extraHeaders[hdrArr[0]] = hdrArr[1]
		}
	}

	inbound := mixed.NewListener(bind, nil, transMode, 300, listener)
	err = inbound.Start()
	if err != nil {
		logger.Fatal(err)
	}

	logger.Info("mixed client started at ", inbound.TCPListener.Addr())

	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)
	<-osSignals

	if config.NetLog != "" {
		engine.StopNetLog()
	}
	engine.Shutdown()
	engine.Destroy()
	inbound.Close()
}

type Listener struct {
	url           string
	authorization string
	engine        cronet.StreamEngine
	extraHeaders  map[string]string
}

func (l *Listener) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	logger.Info(metadata.Source, " => ", metadata.Destination)
	headers := map[string]string{
		"-connect-authority": metadata.Destination.String(),
		"Padding":            generatePaddingHeader(),
	}
	if l.authorization != "" {
		headers["proxy-authorization"] = l.authorization
	}
	for key, value := range l.extraHeaders {
		headers[key] = value
	}
	bidiConn := l.engine.CreateConn(true, false)
	err := bidiConn.Start("CONNECT", l.url, headers, 0, false)
	if err != nil {
		return E.Cause(err, "start bidi conn")
	}
	return bufio.CopyConn(ctx, conn, &PaddingConn{Conn: bidiConn})
}

func (l *Listener) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata M.Metadata) error {
	conn.Close()
	return nil
}

func (l *Listener) HandleError(err error) {
	if E.IsClosed(err) {
		return
	}
	logger.Warn(err)
}

func generatePaddingHeader() string {
	paddingLen := rand.Intn(32) + 30
	padding := make([]byte, paddingLen)
	bits := rand.Uint64()
	for i := 0; i < 16; i++ {
		// Codes that won't be Huffman coded.
		padding[i] = "!#$()+<>?@[]^`{}"[bits&15]
		bits >>= 4
	}
	for i := 16; i < paddingLen; i++ {
		padding[i] = '~'
	}
	return string(padding)
}

const kFirstPaddings = 8

type PaddingConn struct {
	net.Conn

	readPadding      int
	writePadding     int
	readRemaining    int
	paddingRemaining int
}

func (c *PaddingConn) Read(p []byte) (n int, err error) {
	if c.readRemaining > 0 {
		if len(p) > c.readRemaining {
			p = p[:c.readRemaining]
		}
		n, err = c.Read(p)
		if err != nil {
			return
		}
		c.readRemaining -= n
		return
	}
	if c.paddingRemaining > 0 {
		err = rw.SkipN(c.Conn, c.paddingRemaining)
		if err != nil {
			return
		}
		c.readRemaining = 0
	}
	if c.readPadding < kFirstPaddings {
		paddingHdr := p[:3]
		_, err = io.ReadFull(c.Conn, paddingHdr)
		if err != nil {
			return
		}
		originalDataSize := int(binary.BigEndian.Uint16(paddingHdr[:2]))
		paddingSize := int(paddingHdr[3])
		if len(p) > originalDataSize {
			p = p[:originalDataSize]
		}
		n, err = c.Conn.Read(p)
		if err != nil {
			return
		}
		c.readPadding++
		c.readRemaining = originalDataSize - n
		c.paddingRemaining = paddingSize
		return
	}
	return c.Conn.Read(p)
}

func (c *PaddingConn) Write(p []byte) (n int, err error) {
	if c.writePadding < kFirstPaddings {
		paddingSize := rand.Intn(256)
		_buffer := buf.Make(3 + len(p) + paddingSize)
		defer runtime.KeepAlive(_buffer)
		buffer := common.Dup(_buffer)
		binary.BigEndian.PutUint16(buffer, uint16(len(p)))
		buffer[3] = byte(paddingSize)
		copy(buffer[3:], p)
		_, err = c.Conn.Write(buffer)
		if err != nil {
			return
		}
		c.writePadding++
	}
	return c.Conn.Write(p)
}

func (c *PaddingConn) WriteBuffer(buffer *buf.Buffer) error {
	if c.writePadding < kFirstPaddings {
		bufferLen := buffer.Len()
		paddingSize := rand.Intn(256)
		header := buffer.Extend(3)
		binary.BigEndian.PutUint16(header, uint16(bufferLen))
		header[3] = byte(paddingSize)
		buffer.Extend(paddingSize)
		c.writePadding++
	}
	return common.Error(c.Conn.Write(buffer.Bytes()))
}
