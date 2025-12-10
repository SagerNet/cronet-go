package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
// extern CRONET_EXPORT Cronet_RESULT cronetEngineStartWithParams(Cronet_EnginePtr self, Cronet_EngineParamsPtr params);
// extern CRONET_EXPORT bool cronetEngineStartNetLogToFile(Cronet_EnginePtr self, Cronet_String file_name, bool log_all);
// extern CRONET_EXPORT void cronetEngineStopNetLog(Cronet_EnginePtr self);
// extern CRONET_EXPORT Cronet_RESULT cronetEngineShutdown(Cronet_EnginePtr self);
// extern CRONET_EXPORT Cronet_String cronetEngineGetVersionString(Cronet_EnginePtr self);
// extern CRONET_EXPORT Cronet_String cronetEngineGetDefaultUserAgent(Cronet_EnginePtr self);
// extern CRONET_EXPORT void cronetEngineAddRequestFinishedListener(Cronet_EnginePtr self, Cronet_RequestFinishedInfoListenerPtr listener, Cronet_ExecutorPtr executor);
// extern CRONET_EXPORT void cronetEngineRemoveRequestFinishedListener(Cronet_EnginePtr self, Cronet_RequestFinishedInfoListenerPtr listener);
import "C"

import (
	"sync"
	"unsafe"
)

// EngineHandler is an interface for custom Engine implementations (for testing/mocking).
type EngineHandler interface {
	StartWithParams(self Engine, params EngineParams) Result
	StartNetLogToFile(self Engine, fileName string, logAll bool) bool
	StopNetLog(self Engine)
	Shutdown(self Engine) Result
	GetVersionString(self Engine) string
	GetDefaultUserAgent(self Engine) string
	AddRequestFinishedListener(self Engine, listener URLRequestFinishedInfoListener, executor Executor)
	RemoveRequestFinishedListener(self Engine, listener URLRequestFinishedInfoListener)
}

// NewEngineWith creates a new Engine with custom handler (for testing/mocking).
func NewEngineWith(handler EngineHandler) Engine {
	ptr := C.Cronet_Engine_CreateWith(
		(*[0]byte)(C.cronetEngineStartWithParams),
		(*[0]byte)(C.cronetEngineStartNetLogToFile),
		(*[0]byte)(C.cronetEngineStopNetLog),
		(*[0]byte)(C.cronetEngineShutdown),
		(*[0]byte)(C.cronetEngineGetVersionString),
		(*[0]byte)(C.cronetEngineGetDefaultUserAgent),
		(*[0]byte)(C.cronetEngineAddRequestFinishedListener),
		(*[0]byte)(C.cronetEngineRemoveRequestFinishedListener),
	)
	engineHandlerAccess.Lock()
	engineHandlerMap[uintptr(unsafe.Pointer(ptr))] = handler
	engineHandlerAccess.Unlock()
	return Engine{ptr}
}

var (
	engineHandlerAccess sync.RWMutex
	engineHandlerMap    map[uintptr]EngineHandler
)

func init() {
	engineHandlerMap = make(map[uintptr]EngineHandler)
}

func instanceOfEngineHandler(self C.Cronet_EnginePtr) EngineHandler {
	engineHandlerAccess.RLock()
	defer engineHandlerAccess.RUnlock()
	return engineHandlerMap[uintptr(unsafe.Pointer(self))]
}

//export cronetEngineStartWithParams
func cronetEngineStartWithParams(self C.Cronet_EnginePtr, params C.Cronet_EngineParamsPtr) C.Cronet_RESULT {
	handler := instanceOfEngineHandler(self)
	if handler != nil {
		return C.Cronet_RESULT(handler.StartWithParams(Engine{self}, EngineParams{params}))
	}
	return C.Cronet_RESULT_SUCCESS
}

//export cronetEngineStartNetLogToFile
func cronetEngineStartNetLogToFile(self C.Cronet_EnginePtr, fileName C.Cronet_String, logAll C.bool) C.bool {
	handler := instanceOfEngineHandler(self)
	if handler != nil {
		return C.bool(handler.StartNetLogToFile(Engine{self}, C.GoString(fileName), bool(logAll)))
	}
	return C.bool(false)
}

//export cronetEngineStopNetLog
func cronetEngineStopNetLog(self C.Cronet_EnginePtr) {
	handler := instanceOfEngineHandler(self)
	if handler != nil {
		handler.StopNetLog(Engine{self})
	}
}

//export cronetEngineShutdown
func cronetEngineShutdown(self C.Cronet_EnginePtr) C.Cronet_RESULT {
	handler := instanceOfEngineHandler(self)
	if handler != nil {
		return C.Cronet_RESULT(handler.Shutdown(Engine{self}))
	}
	return C.Cronet_RESULT_SUCCESS
}

//export cronetEngineGetVersionString
func cronetEngineGetVersionString(self C.Cronet_EnginePtr) C.Cronet_String {
	handler := instanceOfEngineHandler(self)
	if handler != nil {
		return C.CString(handler.GetVersionString(Engine{self}))
	}
	return nil
}

//export cronetEngineGetDefaultUserAgent
func cronetEngineGetDefaultUserAgent(self C.Cronet_EnginePtr) C.Cronet_String {
	handler := instanceOfEngineHandler(self)
	if handler != nil {
		return C.CString(handler.GetDefaultUserAgent(Engine{self}))
	}
	return nil
}

//export cronetEngineAddRequestFinishedListener
func cronetEngineAddRequestFinishedListener(self C.Cronet_EnginePtr, listener C.Cronet_RequestFinishedInfoListenerPtr, executor C.Cronet_ExecutorPtr) {
	handler := instanceOfEngineHandler(self)
	if handler != nil {
		handler.AddRequestFinishedListener(Engine{self}, URLRequestFinishedInfoListener{listener}, Executor{executor})
	}
}

//export cronetEngineRemoveRequestFinishedListener
func cronetEngineRemoveRequestFinishedListener(self C.Cronet_EnginePtr, listener C.Cronet_RequestFinishedInfoListenerPtr) {
	handler := instanceOfEngineHandler(self)
	if handler != nil {
		handler.RemoveRequestFinishedListener(Engine{self}, URLRequestFinishedInfoListener{listener})
	}
}
