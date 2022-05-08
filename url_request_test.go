package cronet_test

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"

	"github.com/sagernet/cronet-go"
)

func TestURLRequest(t *testing.T) {
	s, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("listen tcp: ", err)
	}
	go func() {
		reqConn, err := s.Accept()
		if err != nil {
			t.Fatal("accept conn: ", err)
		}
		request, err := http.ReadRequest(bufio.NewReader(reqConn))
		if err != nil {
			t.Fatal("read http: ", err)
		}
		response := &http.Response{
			Status:           "200 OK",
			StatusCode:       200,
			Proto:            "HTTP/1.1",
			ProtoMajor:       request.ProtoMajor,
			ProtoMinor:       request.ProtoMinor,
			Request:          request,
			TransferEncoding: nil,
			Close:            true,
		}
		response.Write(reqConn)
		reqConn.Close()
	}()

	e := cronet.NewEngine()
	p := cronet.NewEngineParams()
	p.SetUserAgent("cronet " + e.Version())
	e.StartWithParams(p)
	p.Destroy()

	r := cronet.NewURLRequest()
	rp := cronet.NewURLRequestParams()
	rp.SetMethod("GET")
	rp.SetDisableCache(true)
	h := &urlRequestCallback{t: t}
	h.wg.Add(1)
	ex := cronet.NewExecutor(func(executor cronet.Executor, command cronet.Runnable) {
		go func() {
			command.Run()
			command.Destroy()
		}()
	})
	c := cronet.NewURLRequestCallback(h)
	r.InitWithParams(e, fmt.Sprint("http://", s.Addr()), rp, c, ex)
	rp.Destroy()
	r.Start()
	h.wg.Wait()
	s.Close()
	ex.Destroy()
	e.Shutdown()
	e.Destroy()
}

type urlRequestCallback struct {
	t  *testing.T
	wg sync.WaitGroup
}

func (u *urlRequestCallback) OnRedirectReceived(self cronet.URLRequestCallback, request cronet.URLRequest, info cronet.URLResponseInfo, newLocationUrl string) {
	u.t.Fatal("unexpected redirect")
}

func (u *urlRequestCallback) OnResponseStarted(self cronet.URLRequestCallback, request cronet.URLRequest, info cronet.URLResponseInfo) {
	request.Cancel()
}

func (u *urlRequestCallback) OnReadCompleted(self cronet.URLRequestCallback, request cronet.URLRequest, info cronet.URLResponseInfo, buffer cronet.Buffer, bytesRead int64) {
}

func (u *urlRequestCallback) OnSucceeded(self cronet.URLRequestCallback, request cronet.URLRequest, info cronet.URLResponseInfo) {
}

func (u *urlRequestCallback) OnFailed(self cronet.URLRequestCallback, request cronet.URLRequest, info cronet.URLResponseInfo, error cronet.Error) {
	u.t.Fatal("unexpected failed: ", error.Message())
}

func (u *urlRequestCallback) OnCanceled(self cronet.URLRequestCallback, request cronet.URLRequest, info cronet.URLResponseInfo) {
	u.wg.Done()
}
