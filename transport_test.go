package cronet_test

import (
	"net/http"
	"os"
	"testing"

	"github.com/sagernet/cronet-go"
)

func TestTransport(t *testing.T) {
	client := &http.Client{
		Transport: &cronet.RoundTripper{},
	}
	response, err := client.Get("https://cloudflare.com/cdn-cgi/trace")
	if err != nil {
		t.Fatal(err)
	}
	response.Write(os.Stderr)
	response.Body.Close()
}
