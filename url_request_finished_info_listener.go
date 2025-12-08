package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"

import "unsafe"

// URLRequestFinishedInfoListener
// Listens for finished requests for the purpose of collecting metrics.
type URLRequestFinishedInfoListener struct {
	ptr C.Cronet_RequestFinishedInfoListenerPtr
}

func (l URLRequestFinishedInfoListener) SetClientContext(context unsafe.Pointer) {
	C.Cronet_RequestFinishedInfoListener_SetClientContext(l.ptr, C.Cronet_ClientContext(context))
}

func (l URLRequestFinishedInfoListener) ClientContext() unsafe.Pointer {
	return unsafe.Pointer(C.Cronet_RequestFinishedInfoListener_GetClientContext(l.ptr))
}

// URLRequestFinishedInfoListenerOnRequestFinishedFunc
// Will be called in a task submitted to the Executor passed with
// this URLRequestFinishedInfoListener.
//
// The listener is called before URLRequestCallbackHandler.OnCanceled(),
// URLRequestCallbackHandler.OnFailed() or
// URLRequestCallbackHandler.OnSucceeded() is called -- note that if the executor
// runs the listener asyncronously, the actual call to the listener may
// happen after a URLRequestCallbackHandler method is called.
//
// @param request_info RequestFinishedInfo for finished request.
//
//	Ownership is *not* transferred by this call, do not destroy
//	request_info.
//
//	URLRequest
//	that created it hasn't been destroyed -- **additionally**, it will
//	also always be valid for the duration of URLRequestFinishedInfoListenerOnRequestFinishedFunc(),
//	even if the URLRequest has been destroyed.
//
//	This is accomplished by ownership being shared between the
//	URLRequest and the code that calls this listener.
//
// @param responseInfo A pointer to the same UrlResponseInfo passed to
//
//	URLRequestCallbackHandler.OnCanceled(), {@link
//	URLRequestCallbackHandler.OnFailed()} or {@link
//	URLRequestCallbackHandler.OnSucceeded()}. The lifetime and ownership of
//	requestInfo.
//
// @param error A pointer to the same Error passed to
//
//	URLRequestCallbackHandler.OnFailed(), or null if there was no error.
//	The lifetime and ownership of error works the same as for
//	requestInfo.
//
// /
type URLRequestFinishedInfoListenerOnRequestFinishedFunc func(listener URLRequestFinishedInfoListener, requestInfo URLRequestFinishedInfo, responseInfo URLResponseInfo, error Error)
