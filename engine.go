package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"

import (
	"unsafe"
)

type Engine struct {
	ptr C.Cronet_EnginePtr
}

func NewEngine() Engine {
	return Engine{C.Cronet_Engine_Create()}
}

func (e Engine) Destroy() {
	C.Cronet_Engine_Destroy(e.ptr)
}

func (e Engine) ClientContext() unsafe.Pointer {
	return unsafe.Pointer(C.Cronet_Engine_GetClientContext(e.ptr))
}

func (e Engine) SetClientContext(ctx unsafe.Pointer) {
	C.Cronet_Engine_SetClientContext(e.ptr, C.Cronet_ClientContext(ctx))
}

func (e Engine) StartWithParams(params EngineParameters) Result {
	return Result(C.Cronet_Engine_StartWithParams(e.ptr, params.ptr))
}

func (e Engine) StartNetLogToFile(path string, logAll bool) bool {
	cPath := C.CString(path)
	result := C.Cronet_Engine_StartNetLogToFile(e.ptr, cPath, C.bool(logAll))
	C.free(unsafe.Pointer(cPath))
	return bool(result)
}

func (e Engine) StopNetLog() {
	C.Cronet_Engine_StopNetLog(e.ptr)
}

func (e Engine) Shutdown() Result {
	return Result(C.Cronet_Engine_Shutdown(e.ptr))
}

func (e Engine) Version() string {
	return C.GoString(C.Cronet_Engine_GetVersionString(e.ptr))
}

func (e Engine) DefaultUserAgent() string {
	return C.GoString(C.Cronet_Engine_GetDefaultUserAgent(e.ptr))
}
