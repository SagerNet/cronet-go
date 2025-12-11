package cronet_test

import (
	"fmt"
	"testing"

	"github.com/sagernet/cronet-go"
)

func TestEngineVersion(t *testing.T) {
	params := cronet.NewEngineParams()
	params.SetUserAgent("test")
	engine := cronet.NewEngine()
	engine.StartWithParams(params)
	defer params.Destroy()
	defer engine.Destroy()
	defer engine.Shutdown()

	version := engine.Version()
	fmt.Printf("Cronet Engine Version: %s\n", version)
	if version == "" {
		t.Fatal("engine version is empty")
	}
}
