package cronet_test

import (
	"testing"
	"time"

	"github.com/sagernet/cronet-go"
)

func TestDateTime(t *testing.T) {
	d := cronet.NewDateTime()
	m := time.UnixMilli(time.Now().UnixMilli())
	d.SetValue(m)
	if d.Value() != m {
		t.Fatal("bad time")
	}
	d.Destroy()
}
