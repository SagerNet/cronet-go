package cronet

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

func (e *Engine) Destroy() {
	C.Cronet_Engine_Destroy(e.ptr)
}

func (e *Engine) StartNetLogToFile(path string, logAll bool) {
	cPath := C.CString(path)
	C.Cronet_Engine_StartNetLogToFile(e.ptr, cPath, C.bool(logAll))
	C.free(unsafe.Pointer(cPath))
}

func (e *Engine) StopNetLog() {
	C.Cronet_Engine_StopNetLog(e.ptr)
}

func (e *Engine) Shutdown() {
	C.Cronet_Engine_Shutdown(e.ptr)
}

func (e *Engine) Version() string {
	return C.GoString(C.Cronet_Engine_GetVersionString(e.ptr))
}

func (e *Engine) DefaultUserAgent() string {
	return C.GoString(C.Cronet_Engine_GetDefaultUserAgent(e.ptr))
}
