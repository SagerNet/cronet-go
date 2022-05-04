package main

import (
	"context"
	"encoding/base64"
	"net"
	"net/http"
	"os"

	"github.com/sagernet/cronet-go"
	"github.com/sagernet/sing/common/log"
)

var logger = log.NewLogger("example")

func main() {
	params := cronet.NewEngineParameters()
	params.SetUserAgent("cronet example client")
	params.SetExperimentalOptions(`{"ssl_key_log_file": "/tmp/keys"}`)

	engine := cronet.NewEngine(params)
	engine.StartNetLogToFile("log.json", true)

	streamEngine := engine.StreamEngine()

	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				bidirectionalStream := streamEngine.CreateStream(ctx)
				err := bidirectionalStream.Start("CONNECT", os.Args[1], map[string]string{
					"proxy-authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(os.Args[2])),
					"_real_authority":     addr,
				}, 0, false)
				if err != nil {
					bidirectionalStream.Close()
					return nil, err
				}
				return bidirectionalStream, nil
			},
		},
	}

	response, err := httpClient.Get(os.Args[3])
	if err != nil {
		logger.Error(err)
	} else {
		response.Write(os.Stderr)
	}

	engine.StopNetLog()
}
