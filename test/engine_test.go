package test

import (
	"testing"
	"time"

	cronet "github.com/sagernet/cronet-go"

	"github.com/stretchr/testify/require"
)

func TestDateTime(t *testing.T) {
	dateTime := cronet.NewDateTime()
	defer dateTime.Destroy()

	now := time.UnixMilli(time.Now().UnixMilli())
	dateTime.SetValue(now)
	require.Equal(t, now, dateTime.Value())
}

func TestEngineVersion(t *testing.T) {
	params := cronet.NewEngineParams()
	params.SetUserAgent("test")
	defer params.Destroy()

	engine := cronet.NewEngine()
	require.Equal(t, cronet.ResultSuccess, engine.StartWithParams(params))
	defer engine.Destroy()
	defer engine.Shutdown()

	require.Equal(t, "150.0.7871.63", engine.Version())
}
