package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
// extern CRONET_EXPORT void cronetURLRequestStatusListenerOnStatus(Cronet_UrlRequestStatusListenerPtr self, Cronet_UrlRequestStatusListener_Status status);
import "C"

import (
	"sync"
	"unsafe"
)

func NewURLRequestStatusListener(onStatusFunc URLRequestStatusListenerOnStatusFunc) URLRequestStatusListener {
	ptr := C.Cronet_UrlRequestStatusListener_CreateWith((*[0]byte)(C.cronetURLRequestStatusListenerOnStatus))
	urlRequestStatusListenerAccess.Lock()
	urlRequestStatusListenerMap[uintptr(unsafe.Pointer(ptr))] = onStatusFunc
	urlRequestStatusListenerAccess.Unlock()
	return URLRequestStatusListener{ptr}
}

func (l URLRequestStatusListener) Destroy() {
	C.Cronet_UrlRequestStatusListener_Destroy(l.ptr)
	urlRequestStatusListenerAccess.Lock()
	delete(urlRequestStatusListenerMap, uintptr(unsafe.Pointer(l.ptr)))
	urlRequestStatusListenerAccess.Unlock()
}

var (
	urlRequestStatusListenerAccess sync.RWMutex
	urlRequestStatusListenerMap    map[uintptr]URLRequestStatusListenerOnStatusFunc
)

func init() {
	urlRequestStatusListenerMap = make(map[uintptr]URLRequestStatusListenerOnStatusFunc)
}

//export cronetURLRequestStatusListenerOnStatus
func cronetURLRequestStatusListenerOnStatus(self C.Cronet_UrlRequestStatusListenerPtr, status C.Cronet_UrlRequestStatusListener_Status) {
	ptr := uintptr(unsafe.Pointer(self))
	urlRequestStatusListenerAccess.Lock()
	listener := urlRequestStatusListenerMap[ptr]
	delete(urlRequestStatusListenerMap, ptr)
	urlRequestStatusListenerAccess.Unlock()
	if listener == nil {
		panic("nil url status listener")
	}
	listener(URLRequestStatusListener{self}, URLRequestStatusListenerStatus(status))
}
