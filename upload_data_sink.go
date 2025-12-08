package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"
import "unsafe"

// UploadDataSink
//
// Defines callbacks methods for UploadDataProvider. All methods
// may be called synchronously or asynchronously, on any thread.
type UploadDataSink struct {
	ptr C.Cronet_UploadDataSinkPtr
}

// OnReadSucceeded
//
// Called by UploadDataProviderHandler when a read succeeds.
//
// @param bytesRead number of bytes read into buffer passed to UploadDataProviderHandler.Read().
// @param finalChunk For chunked uploads, |true| if this is the final
//     read. It must be |false| for non-chunked uploads.
func (s UploadDataSink) OnReadSucceeded(bytesRead int64, finalChunk bool) {
	C.Cronet_UploadDataSink_OnReadSucceeded(s.ptr, C.uint64_t(bytesRead), C.bool(finalChunk))
}

// OnReadError
// Called by UploadDataProviderHandler when a read fails.
// @param message to pass on to URLRequestCallbackHandler.OnFailed().
func (s UploadDataSink) OnReadError(message string) {
	cMessage := C.CString(message)
	C.Cronet_UploadDataSink_OnReadError(s.ptr, cMessage)
	C.free(unsafe.Pointer(cMessage))
}

// OnRewindSucceeded
// Called by UploadDataProviderHandler when a rewind succeeds.
func (s UploadDataSink) OnRewindSucceeded() {
	C.Cronet_UploadDataSink_OnRewindSucceeded(s.ptr)
}

// OnRewindError
// Called by UploadDataProviderHandler when a rewind fails, or if rewinding
// uploads is not supported.
// * @param message to pass on to URLRequestCallbackHandler.OnFailed().
func (s UploadDataSink) OnRewindError(message string) {
	cMessage := C.CString(message)
	C.Cronet_UploadDataSink_OnRewindError(s.ptr, cMessage)
	C.free(unsafe.Pointer(cMessage))
}

func (s UploadDataSink) SetClientContext(context unsafe.Pointer) {
	C.Cronet_UploadDataSink_SetClientContext(s.ptr, C.Cronet_ClientContext(context))
}

func (s UploadDataSink) ClientContext() unsafe.Pointer {
	return unsafe.Pointer(C.Cronet_UploadDataSink_GetClientContext(s.ptr))
}
