package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"
import "unsafe"

type URLRequestParamsRequestPriority int

const (
	// URLRequestParamsRequestPriorityIdle
	// Lowest request priority.
	URLRequestParamsRequestPriorityIdle URLRequestParamsRequestPriority = 0

	// URLRequestParamsRequestPriorityLowest
	// Very low request priority.
	URLRequestParamsRequestPriorityLowest URLRequestParamsRequestPriority = 1

	// URLRequestParamsRequestPriorityLow
	// Low request priority.
	URLRequestParamsRequestPriorityLow URLRequestParamsRequestPriority = 2

	// URLRequestParamsRequestPriorityMedium
	// Medium request priority. This is the default priority given to the request.
	URLRequestParamsRequestPriorityMedium URLRequestParamsRequestPriority = 3

	// URLRequestParamsRequestPriorityHighest
	// Highest request priority.
	URLRequestParamsRequestPriorityHighest URLRequestParamsRequestPriority = 4
)

// URLRequestParams
// Parameters for UrlRequest. Allows configuring requests before initializing them
// with URLRequest.InitWithParams().
type URLRequestParams struct {
	ptr C.Cronet_UrlRequestParamsPtr
}

func NewURLRequestParams() URLRequestParams {
	return URLRequestParams{C.Cronet_UrlRequestParams_Create()}
}

func (p URLRequestParams) Destroy() {
	C.Cronet_UrlRequestParams_Destroy(p.ptr)
}

// SetMethod
// The HTTP method verb to use for this request.
//
// The default when this value is not set is "GET" if the request has
// no body or "POST" if it does.
//
// Allowed methods are "GET", "HEAD", "DELETE", "POST", "PUT" and "CONNECT".
func (p URLRequestParams) SetMethod(method string) {
	cMethod := C.CString(method)
	C.Cronet_UrlRequestParams_http_method_set(p.ptr, cMethod)
	C.free(unsafe.Pointer(cMethod))
}

func (p URLRequestParams) Method() string {
	return C.GoString(C.Cronet_UrlRequestParams_http_method_get(p.ptr))
}

// AddHeader
// Add HTTP header for this request
func (p URLRequestParams) AddHeader(header HTTPHeader) {
	C.Cronet_UrlRequestParams_request_headers_add(p.ptr, header.ptr)
}

func (p URLRequestParams) HeaderSize() int {
	return int(C.Cronet_UrlRequestParams_request_headers_size(p.ptr))
}

func (p URLRequestParams) HeaderAt(index int) HTTPHeader {
	return HTTPHeader{C.Cronet_UrlRequestParams_request_headers_at(p.ptr, C.uint32_t(index))}
}

func (p URLRequestParams) ClearHeaders() {
	C.Cronet_UrlRequestParams_request_headers_clear(p.ptr)
}

// SetDisableCache
// Disables cache for the request. If context is not set up to use cache,
// this call has no effect.
func (p URLRequestParams) SetDisableCache(disable bool) {
	C.Cronet_UrlRequestParams_disable_cache_set(p.ptr, C.bool(disable))
}

func (p URLRequestParams) DisableCache() bool {
	return bool(C.Cronet_UrlRequestParams_disable_cache_get(p.ptr))
}

// SetPriority
// Priority of the request which should be one of the URLRequestParamsRequestPriority values.
func (p URLRequestParams) SetPriority(priority URLRequestParamsRequestPriority) {
	C.Cronet_UrlRequestParams_priority_set(p.ptr, C.Cronet_UrlRequestParams_REQUEST_PRIORITY(priority))
}

func (p URLRequestParams) Priority() URLRequestParamsRequestPriority {
	return URLRequestParamsRequestPriority(C.Cronet_UrlRequestParams_priority_get(p.ptr))
}

// SetUploadDataProvider
// Upload data provider. Setting this value switches method to "POST" if not
// explicitly set. Starting the request will fail if a Content-Type header is not set.
func (p URLRequestParams) SetUploadDataProvider(provider UploadDataProvider) {
	C.Cronet_UrlRequestParams_upload_data_provider_set(p.ptr, provider.ptr)
}

func (p URLRequestParams) UploadDataProvider() UploadDataProvider {
	return UploadDataProvider{C.Cronet_UrlRequestParams_upload_data_provider_get(p.ptr)}
}

// SetUploadDataExecutor
// Upload data provider executor that will be used to invoke uploadDataProvider.
func (p URLRequestParams) SetUploadDataExecutor(executor Executor) {
	C.Cronet_UrlRequestParams_upload_data_provider_executor_set(p.ptr, executor.ptr)
}

func (p URLRequestParams) UploadDataExecutor() Executor {
	return Executor{C.Cronet_UrlRequestParams_upload_data_provider_executor_get(p.ptr)}
}

// SetAllowDirectExecutor
// Marks that the executors this request will use to notify callbacks (for
// UploadDataProvider and URLRequestCallback) is intentionally performing
// inline execution without switching to another thread.
//
// <p><b>Warning:</b> This option makes it easy to accidentally block the network thread.
// It should not be used if your callbacks perform disk I/O, acquire locks, or call into
// other code you don't carefully control and audit.
func (p URLRequestParams) SetAllowDirectExecutor(allow bool) {
	C.Cronet_UrlRequestParams_allow_direct_executor_set(p.ptr, C.bool(allow))
}

func (p URLRequestParams) AllocDirectExecutor() bool {
	return bool(C.Cronet_UrlRequestParams_allow_direct_executor_get(p.ptr))
}

// AddAnnotation
// Associates the annotation object with this request. May add more than one.
// Passed through to a RequestFinishedInfoListener.
func (p URLRequestParams) AddAnnotation(annotation unsafe.Pointer) {
	C.Cronet_UrlRequestParams_annotations_add(p.ptr, C.Cronet_RawDataPtr(annotation))
}

func (p URLRequestParams) AnnotationSize() int {
	return int(C.Cronet_UrlRequestParams_annotations_size(p.ptr))
}

func (p URLRequestParams) AnnotationAt(index int) unsafe.Pointer {
	return unsafe.Pointer(C.Cronet_UrlRequestParams_annotations_at(p.ptr, C.uint32_t(index)))
}

func (p URLRequestParams) ClearAnnotations() {
	C.Cronet_UrlRequestParams_annotations_clear(p.ptr)
}

// SetRequestFinishedListener
// A listener that gets invoked at the end of each request.
//
// The listener is invoked with the request finished info on RequestFinishedExecutor, which must be set.
//
// The listener is called before URLRequestCallbackHandler.OnCanceled(),
// URLRequestCallbackHandler.OnFailed() or
// URLRequestCallbackHandler.OnSucceeded() is called -- note that if
// RequestFinishedListener runs the listener asynchronously, the actual
// call to the listener may happen after a {@code URLRequestCallbackHandler} method
// is called.
//
// Ownership is **not** taken.
//
// Assuming the listener won't run again (there are no pending requests with
// the listener attached, either via Engine or @code URLRequest),
// the app may destroy it once its URLRequestFinishedInfoListenerOnRequestFinishedFunc has started,
// even inside that method.
func (p URLRequestParams) SetRequestFinishedListener(listener URLRequestFinishedInfoListener) {
	C.Cronet_UrlRequestParams_request_finished_listener_set(p.ptr, listener.ptr)
}

func (p URLRequestParams) RequestFinishedListener() URLRequestFinishedInfoListener {
	return URLRequestFinishedInfoListener{C.Cronet_UrlRequestParams_request_finished_listener_get(p.ptr)}
}

// SetRequestFinishedExecutor
// The Executor used to run the RequestFinishedListener.
//
// Ownership is **not** taken.
//
// # Similar to RequestFinishedListener, the app may destroy RequestFinishedExecutor in or after URLRequestFinishedInfoListenerOnRequestFinishedFunc
//
// It's also OK to destroy RequestFinishedExecutor in or after one
// of {@link URLRequestCallbackHandler.OnCanceled()}, {@link
// URLRequestCallbackHandler.OnFailed()} or {@link
// URLRequestCallbackHandler.OnSucceeded()}.
//
// Of course, both of these are only true if {@code
// request_finished_executor} isn't being used for anything else that might
// start running in the future.
func (p URLRequestParams) SetRequestFinishedExecutor(executor Executor) {
	C.Cronet_UrlRequestParams_request_finished_executor_set(p.ptr, executor.ptr)
}

func (p URLRequestParams) RequestFinishedExecutor() Executor {
	return Executor{C.Cronet_UrlRequestParams_request_finished_executor_get(p.ptr)}
}

type URLRequestParamsIdempotency int

const (
	URLRequestParamsIdempotencyDefaultIdempotency URLRequestParamsIdempotency = 0
	URLRequestParamsIdempotencyIdempotent         URLRequestParamsIdempotency = 1
	URLRequestParamsIdempotencyNotIdempotent      URLRequestParamsIdempotency = 2
)

// SetIdempotency
// Idempotency of the request, which determines that if it is safe to enable
// 0-RTT for the Cronet request. By default, 0-RTT is only enabled for safe
// HTTP methods, i.e., GET, HEAD, OPTIONS, and TRACE. For other methods,
// enabling 0-RTT may cause security issues since a network observer can
// replay the request. If the request has any side effects, those effects can
// happen multiple times. It is only safe to enable the 0-RTT if it is known
// that the request is idempotent.
func (p URLRequestParams) SetIdempotency(idempotency URLRequestParamsIdempotency) {
	C.Cronet_UrlRequestParams_idempotency_set(p.ptr, C.Cronet_UrlRequestParams_IDEMPOTENCY(idempotency))
}

func (p URLRequestParams) Idempotency() URLRequestParamsIdempotency {
	return URLRequestParamsIdempotency(C.Cronet_UrlRequestParams_idempotency_get(p.ptr))
}
