package cronet_test

import (
	"testing"

	"github.com/sagernet/cronet-go"
)

func TestExecutor(t *testing.T) {
	e := cronet.NewExecutor(func(executor cronet.Executor, command cronet.Runnable) {
		go func() {
			command.Run()
			command.Destroy()
		}()
	})
	e.Destroy()
}
