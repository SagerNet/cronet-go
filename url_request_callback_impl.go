package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
// extern void cronetURLRequestCallbackOnRedirectReceived(Cronet_UrlRequestCallbackPtr self, Cronet_UrlRequestPtr request, Cronet_UrlResponseInfoPtr info, Cronet_String new_location_url);
// extern void cronetURLRequestCallbackOnResponseStarted(Cronet_UrlRequestCallbackPtr self, Cronet_UrlRequestPtr request, Cronet_UrlResponseInfoPtr info);
// extern void cronetURLRequestCallbackOnReadCompleted(Cronet_UrlRequestCallbackPtr self, Cronet_UrlRequestPtr request, Cronet_UrlResponseInfoPtr info, Cronet_BufferPtr buffer, uint64_t bytes_read);
// extern void cronetURLRequestCallbackOnSucceeded(Cronet_UrlRequestCallbackPtr self, Cronet_UrlRequestPtr request, Cronet_UrlResponseInfoPtr info);
// extern void cronetURLRequestCallbackOnFailed(Cronet_UrlRequestCallbackPtr self, Cronet_UrlRequestPtr request, Cronet_UrlResponseInfoPtr info, Cronet_ErrorPtr error);
// extern void cronetURLRequestCallbackOnCanceled(Cronet_UrlRequestCallbackPtr self, Cronet_UrlRequestPtr request, Cronet_UrlResponseInfoPtr info);
import "C"

import (
	"sync"
	"unsafe"
)

func NewURLRequestCallback(handler URLRequestCallbackHandler) URLRequestCallback {
	ptr := C.Cronet_UrlRequestCallback_CreateWith(
		(*[0]byte)(C.cronetURLRequestCallbackOnRedirectReceived),
		(*[0]byte)(C.cronetURLRequestCallbackOnResponseStarted),
		(*[0]byte)(C.cronetURLRequestCallbackOnReadCompleted),
		(*[0]byte)(C.cronetURLRequestCallbackOnSucceeded),
		(*[0]byte)(C.cronetURLRequestCallbackOnFailed),
		(*[0]byte)(C.cronetURLRequestCallbackOnCanceled),
	)
	urlRequestCallbackAccess.Lock()
	urlRequestCallbackMap[uintptr(unsafe.Pointer(ptr))] = handler
	urlRequestCallbackAccess.Unlock()
	return URLRequestCallback{ptr}
}

func (l URLRequestCallback) Destroy() {
	C.Cronet_UrlRequestCallback_Destroy(l.ptr)
	urlRequestCallbackAccess.Lock()
	delete(urlRequestCallbackMap, uintptr(unsafe.Pointer(l.ptr)))
	urlRequestCallbackAccess.RUnlock()
}

var (
	urlRequestCallbackAccess sync.RWMutex
	urlRequestCallbackMap    map[uintptr]URLRequestCallbackHandler
)

func init() {
	urlRequestCallbackMap = make(map[uintptr]URLRequestCallbackHandler)
}

func instanceOfURLRequestCallback(self C.Cronet_UrlRequestCallbackPtr) URLRequestCallbackHandler {
	urlRequestCallbackAccess.RLock()
	defer urlRequestCallbackAccess.RUnlock()
	callback := urlRequestCallbackMap[uintptr(unsafe.Pointer(self))]
	if callback == nil {
		panic("nil url request callback")
	}
	return callback
}

//export cronetURLRequestCallbackOnRedirectReceived
func cronetURLRequestCallbackOnRedirectReceived(self C.Cronet_UrlRequestCallbackPtr, request C.Cronet_UrlRequestPtr, info C.Cronet_UrlResponseInfoPtr, newLocationUrl C.Cronet_String) {
	instanceOfURLRequestCallback(self).OnRedirectReceived(URLRequestCallback{self}, URLRequest{request}, URLResponseInfo{info}, C.GoString(newLocationUrl))
}

//export cronetURLRequestCallbackOnResponseStarted
func cronetURLRequestCallbackOnResponseStarted(self C.Cronet_UrlRequestCallbackPtr, request C.Cronet_UrlRequestPtr, info C.Cronet_UrlResponseInfoPtr) {
	instanceOfURLRequestCallback(self).OnResponseStarted(URLRequestCallback{self}, URLRequest{request}, URLResponseInfo{info})
}

//export cronetURLRequestCallbackOnReadCompleted
func cronetURLRequestCallbackOnReadCompleted(self C.Cronet_UrlRequestCallbackPtr, request C.Cronet_UrlRequestPtr, info C.Cronet_UrlResponseInfoPtr, buffer C.Cronet_BufferPtr, bytesRead C.uint64_t) {
	instanceOfURLRequestCallback(self).OnReadCompleted(URLRequestCallback{self}, URLRequest{request}, URLResponseInfo{info}, Buffer{buffer}, int64(bytesRead))
}

//export cronetURLRequestCallbackOnSucceeded
func cronetURLRequestCallbackOnSucceeded(self C.Cronet_UrlRequestCallbackPtr, request C.Cronet_UrlRequestPtr, info C.Cronet_UrlResponseInfoPtr) {
	instanceOfURLRequestCallback(self).OnSucceeded(URLRequestCallback{self}, URLRequest{request}, URLResponseInfo{info})
}

//export cronetURLRequestCallbackOnFailed
func cronetURLRequestCallbackOnFailed(self C.Cronet_UrlRequestCallbackPtr, request C.Cronet_UrlRequestPtr, info C.Cronet_UrlResponseInfoPtr, error C.Cronet_ErrorPtr) {
	instanceOfURLRequestCallback(self).OnFailed(URLRequestCallback{self}, URLRequest{request}, URLResponseInfo{info}, Error{error})
}

//export cronetURLRequestCallbackOnCanceled
func cronetURLRequestCallbackOnCanceled(self C.Cronet_UrlRequestCallbackPtr, request C.Cronet_UrlRequestPtr, info C.Cronet_UrlResponseInfoPtr) {
	instanceOfURLRequestCallback(self).OnCanceled(URLRequestCallback{self}, URLRequest{request}, URLResponseInfo{info})
}
