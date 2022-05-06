package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
// extern void cronetBufferCallbackOnDestroy(Cronet_BufferCallbackPtr self,Cronet_BufferPtr buffer);
import "C"

import (
	"sync"
	"unsafe"
)

func NewBufferCallback(callbackFunc BufferCallbackFunc) BufferCallback {
	ptr := C.Cronet_BufferCallback_CreateWith((*[0]byte)(C.cronetBufferCallbackOnDestroy))
	if callbackFunc != nil {
		bufferCallbackAccess.Lock()
		bufferCallbackMap[uintptr(unsafe.Pointer(ptr))] = callbackFunc
		bufferCallbackAccess.Unlock()
	}
	return BufferCallback{ptr}
}

var (
	bufferCallbackAccess sync.Mutex
	bufferCallbackMap    map[uintptr]BufferCallbackFunc
)

func init() {
	bufferCallbackMap = make(map[uintptr]BufferCallbackFunc)
}

//export cronetBufferCallbackOnDestroy
func cronetBufferCallbackOnDestroy(self C.Cronet_BufferCallbackPtr, buffer C.Cronet_BufferPtr) {
	ptrInt := uintptr(unsafe.Pointer(self))
	bufferCallbackAccess.Lock()
	callback := bufferCallbackMap[ptrInt]
	delete(bufferCallbackMap, ptrInt)
	bufferCallbackAccess.Unlock()
	if callback != nil {
		callback(BufferCallback{self}, Buffer{buffer})
	}
}
