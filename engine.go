package cronet

// #cgo CFLAGS: -I.
// #cgo LDFLAGS: -L. -lcronet.100.0.4896.60
// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"
import "unsafe"

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

func (e *Engine) StartNetLogToFile(path string, logAll bool) {
	cPath := C.CString(path)
	C.Cronet_Engine_StartNetLogToFile(e.ptr, cPath, C.bool(logAll))
	C.free(unsafe.Pointer(cPath))
}

func (e *Engine) StopNetLog() {
	C.Cronet_Engine_StopNetLog(e.ptr)
}
