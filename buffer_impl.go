package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
// extern void cronetBufferInitWithDataAndCallback(Cronet_BufferPtr self, Cronet_RawDataPtr data, uint64_t size, Cronet_BufferCallbackPtr callback);
// extern void cronetBufferInitWithAlloc(Cronet_BufferPtr self, uint64_t size);
// extern uint64_t cronetBufferGetSize(Cronet_BufferPtr self);
// extern Cronet_RawDataPtr cronetBufferGetData(Cronet_BufferPtr self);
import "C"

import (
	"sync"
	"unsafe"
)

// BufferHandler is an interface for custom Buffer implementations (for testing/mocking).
type BufferHandler interface {
	InitWithDataAndCallback(self Buffer, data unsafe.Pointer, size uint64, callback BufferCallback)
	InitWithAlloc(self Buffer, size uint64)
	GetSize(self Buffer) uint64
	GetData(self Buffer) unsafe.Pointer
}

// NewBufferWith creates a new Buffer with custom handler (for testing/mocking).
func NewBufferWith(handler BufferHandler) Buffer {
	ptr := C.Cronet_Buffer_CreateWith(
		(*[0]byte)(C.cronetBufferInitWithDataAndCallback),
		(*[0]byte)(C.cronetBufferInitWithAlloc),
		(*[0]byte)(C.cronetBufferGetSize),
		(*[0]byte)(C.cronetBufferGetData),
	)
	bufferHandlerAccess.Lock()
	bufferHandlerMap[uintptr(unsafe.Pointer(ptr))] = handler
	bufferHandlerAccess.Unlock()
	return Buffer{ptr}
}

var (
	bufferHandlerAccess sync.RWMutex
	bufferHandlerMap    map[uintptr]BufferHandler
)

func init() {
	bufferHandlerMap = make(map[uintptr]BufferHandler)
}

func instanceOfBufferHandler(self C.Cronet_BufferPtr) BufferHandler {
	bufferHandlerAccess.RLock()
	defer bufferHandlerAccess.RUnlock()
	return bufferHandlerMap[uintptr(unsafe.Pointer(self))]
}

//export cronetBufferInitWithDataAndCallback
func cronetBufferInitWithDataAndCallback(self C.Cronet_BufferPtr, data C.Cronet_RawDataPtr, size C.uint64_t, callback C.Cronet_BufferCallbackPtr) {
	handler := instanceOfBufferHandler(self)
	if handler != nil {
		handler.InitWithDataAndCallback(Buffer{self}, unsafe.Pointer(data), uint64(size), BufferCallback{callback})
	}
}

//export cronetBufferInitWithAlloc
func cronetBufferInitWithAlloc(self C.Cronet_BufferPtr, size C.uint64_t) {
	handler := instanceOfBufferHandler(self)
	if handler != nil {
		handler.InitWithAlloc(Buffer{self}, uint64(size))
	}
}

//export cronetBufferGetSize
func cronetBufferGetSize(self C.Cronet_BufferPtr) C.uint64_t {
	handler := instanceOfBufferHandler(self)
	if handler != nil {
		return C.uint64_t(handler.GetSize(Buffer{self}))
	}
	return 0
}

//export cronetBufferGetData
func cronetBufferGetData(self C.Cronet_BufferPtr) C.Cronet_RawDataPtr {
	handler := instanceOfBufferHandler(self)
	if handler != nil {
		return C.Cronet_RawDataPtr(handler.GetData(Buffer{self}))
	}
	return nil
}
