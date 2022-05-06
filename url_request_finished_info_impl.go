package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
// extern void cronetURLRequestFinishedInfoListenerOnRequestFinished(Cronet_RequestFinishedInfoListenerPtr self, Cronet_RequestFinishedInfoPtr request_info, Cronet_UrlResponseInfoPtr response_info, Cronet_ErrorPtr error);
import "C"

import (
	"sync"
	"unsafe"
)

func NewURLRequestFinishedInfoListener(finishedFunc URLRequestFinishedInfoListenerOnRequestFinishedFunc) URLRequestFinishedInfoListener {
	ptr := C.Cronet_RequestFinishedInfoListener_CreateWith((*[0]byte)(C.cronetURLRequestFinishedInfoListenerOnRequestFinished))
	urlRequestFinishedInfoListenerAccess.Lock()
	urlRequestFinishedInfoListenerMap[uintptr(unsafe.Pointer(ptr))] = finishedFunc
	urlRequestFinishedInfoListenerAccess.Unlock()
	return URLRequestFinishedInfoListener{ptr}
}

func (l URLRequestFinishedInfoListener) Destroy() {
	C.Cronet_RequestFinishedInfoListener_Destroy(l.ptr)
}

var (
	urlRequestFinishedInfoListenerAccess sync.RWMutex
	urlRequestFinishedInfoListenerMap    map[uintptr]URLRequestFinishedInfoListenerOnRequestFinishedFunc
)

func init() {
	urlRequestFinishedInfoListenerMap = make(map[uintptr]URLRequestFinishedInfoListenerOnRequestFinishedFunc)
}

//export cronetURLRequestFinishedInfoListenerOnRequestFinished
func cronetURLRequestFinishedInfoListenerOnRequestFinished(self C.Cronet_RequestFinishedInfoListenerPtr, requestInfo C.Cronet_RequestFinishedInfoPtr, responseInfo C.Cronet_UrlResponseInfoPtr, error C.Cronet_ErrorPtr) {
	urlRequestFinishedInfoListenerAccess.RLock()
	listener := urlRequestFinishedInfoListenerMap[uintptr(unsafe.Pointer(self))]
	urlRequestFinishedInfoListenerAccess.RUnlock()
	if listener == nil {
		panic("nil url request finished info listener")
	}
	listener(URLRequestFinishedInfoListener{self}, URLRequestFinishedInfo{requestInfo}, URLResponseInfo{responseInfo}, Error{error})
}
