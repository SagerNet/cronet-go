package main

import (
	"context"
	"encoding/base64"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/sagernet/cronet-go"
)

func main() {
	params := cronet.NewEngineParameters()
	params.SetUserAgent("cronet example client")
	params.SetExperimentalOptions(`{"ssl_key_log_file": "/tmp/keys"}`)

	engine := cronet.NewEngine()
	engine.StartWithParams(params)
	params.Destroy()

	log.Println("libcronet ", engine.Version())

	if len(os.Args) < 4 {
		log.Fatalln("missing args")
	}

	engine.StartNetLogToFile("log.json", true)

	streamEngine := engine.StreamEngine()

	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				conn := streamEngine.CreateConn(true, false)
				err := conn.Start("CONNECT", os.Args[1], map[string]string{
					"proxy-authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(os.Args[2])),
					"-connect-authority":  addr,
				}, 0, false)
				if err != nil {
					conn.Close()
					return nil, err
				}
				return conn, nil
			},
		},
	}

	response, err := httpClient.Get(os.Args[3])
	if err != nil {
		log.Println(err)
	} else {
		response.Write(os.Stderr)
	}

	engine.StopNetLog()
	engine.Shutdown()
	engine.Destroy()
}
