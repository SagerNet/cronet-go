package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
// extern void cronetUploadDataSinkOnReadSucceeded(Cronet_UploadDataSinkPtr self, uint64_t bytes_read, bool final_chunk);
// extern void cronetUploadDataSinkOnReadError(Cronet_UploadDataSinkPtr self, Cronet_String error_message);
// extern void cronetUploadDataSinkOnRewindSucceeded(Cronet_UploadDataSinkPtr self);
// extern void cronetUploadDataSinkOnRewindError(Cronet_UploadDataSinkPtr self, Cronet_String error_message);
import "C"

import (
	"sync"
	"unsafe"
)

// UploadDataSinkHandler is an interface for custom UploadDataSink implementations (for testing/mocking).
type UploadDataSinkHandler interface {
	OnReadSucceeded(self UploadDataSink, bytesRead uint64, finalChunk bool)
	OnReadError(self UploadDataSink, errorMessage string)
	OnRewindSucceeded(self UploadDataSink)
	OnRewindError(self UploadDataSink, errorMessage string)
}

// NewUploadDataSinkWith creates a new UploadDataSink with custom handler (for testing/mocking).
func NewUploadDataSinkWith(handler UploadDataSinkHandler) UploadDataSink {
	ptr := C.Cronet_UploadDataSink_CreateWith(
		(*[0]byte)(C.cronetUploadDataSinkOnReadSucceeded),
		(*[0]byte)(C.cronetUploadDataSinkOnReadError),
		(*[0]byte)(C.cronetUploadDataSinkOnRewindSucceeded),
		(*[0]byte)(C.cronetUploadDataSinkOnRewindError),
	)
	uploadDataSinkHandlerAccess.Lock()
	uploadDataSinkHandlerMap[uintptr(unsafe.Pointer(ptr))] = handler
	uploadDataSinkHandlerAccess.Unlock()
	return UploadDataSink{ptr}
}

var (
	uploadDataSinkHandlerAccess sync.RWMutex
	uploadDataSinkHandlerMap    map[uintptr]UploadDataSinkHandler
)

func init() {
	uploadDataSinkHandlerMap = make(map[uintptr]UploadDataSinkHandler)
}

func instanceOfUploadDataSinkHandler(self C.Cronet_UploadDataSinkPtr) UploadDataSinkHandler {
	uploadDataSinkHandlerAccess.RLock()
	defer uploadDataSinkHandlerAccess.RUnlock()
	return uploadDataSinkHandlerMap[uintptr(unsafe.Pointer(self))]
}

//export cronetUploadDataSinkOnReadSucceeded
func cronetUploadDataSinkOnReadSucceeded(self C.Cronet_UploadDataSinkPtr, bytesRead C.uint64_t, finalChunk C.bool) {
	handler := instanceOfUploadDataSinkHandler(self)
	if handler != nil {
		handler.OnReadSucceeded(UploadDataSink{self}, uint64(bytesRead), bool(finalChunk))
	}
}

//export cronetUploadDataSinkOnReadError
func cronetUploadDataSinkOnReadError(self C.Cronet_UploadDataSinkPtr, errorMessage C.Cronet_String) {
	handler := instanceOfUploadDataSinkHandler(self)
	if handler != nil {
		handler.OnReadError(UploadDataSink{self}, C.GoString(errorMessage))
	}
}

//export cronetUploadDataSinkOnRewindSucceeded
func cronetUploadDataSinkOnRewindSucceeded(self C.Cronet_UploadDataSinkPtr) {
	handler := instanceOfUploadDataSinkHandler(self)
	if handler != nil {
		handler.OnRewindSucceeded(UploadDataSink{self})
	}
}

//export cronetUploadDataSinkOnRewindError
func cronetUploadDataSinkOnRewindError(self C.Cronet_UploadDataSinkPtr, errorMessage C.Cronet_String) {
	handler := instanceOfUploadDataSinkHandler(self)
	if handler != nil {
		handler.OnRewindError(UploadDataSink{self}, C.GoString(errorMessage))
	}
}
