package cronet

// #cgo CFLAGS: -I.
// #cgo LDFLAGS: -L. -lcronet.100.0.4896.60
// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"

type Engine struct {
	ptr C.Cronet_EnginePtr
}

func NewEngine(parameters *EngineParameters) *Engine {
	cronetEngine := C.Cronet_Engine_Create()
	C.Cronet_Engine_StartWithParams(cronetEngine, parameters.ptr)
	parameters.Destroy()
	return &Engine{
		cronetEngine,
	}
}
