package cronet_test

import (
	"testing"

	"github.com/sagernet/cronet-go"
)

func TestEngine(t *testing.T) {
	e := cronet.NewEngine()
	p := cronet.NewEngineParams()
	p.SetUserAgent("test")
	e.StartWithParams(p)
	p.Destroy()
	e.Shutdown()
	e.Destroy()
}
