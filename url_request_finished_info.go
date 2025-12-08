package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"

import "unsafe"

// URLRequestFinishedInfo
// Information about a finished request.
type URLRequestFinishedInfo struct {
	ptr C.Cronet_RequestFinishedInfoPtr
}

func NewURLRequestFinishedInfo() URLRequestFinishedInfo {
	return URLRequestFinishedInfo{C.Cronet_RequestFinishedInfo_Create()}
}

func (i URLRequestFinishedInfo) Destroy() {
	C.Cronet_RequestFinishedInfo_Destroy(i.ptr)
}

// URLRequestFinishedInfoFinishedReason
// The reason why the request finished.
type URLRequestFinishedInfoFinishedReason int

const (
	// URLRequestFinishedInfoFinishedReasonSucceeded
	// The request succeeded.
	URLRequestFinishedInfoFinishedReasonSucceeded URLRequestFinishedInfoFinishedReason = 0

	// URLRequestFinishedInfoFinishedReasonFailed
	// The request failed or returned an error.
	URLRequestFinishedInfoFinishedReasonFailed URLRequestFinishedInfoFinishedReason = 1

	// URLRequestFinishedInfoFinishedReasonCanceled
	// The request was canceled.
	URLRequestFinishedInfoFinishedReasonCanceled URLRequestFinishedInfoFinishedReason = 2
)

// Metrics
// Metrics collected for this request.
func (i URLRequestFinishedInfo) Metrics() Metrics {
	return Metrics{C.Cronet_RequestFinishedInfo_metrics_get(i.ptr)}
}

// AnnotationSize
// The objects that the caller has supplied when initiating the request,
// using URLRequestParams.AddAnnotation
//
// Annotations can be used to associate a RequestFinishedInfo with
// the original request or type of request.
func (i URLRequestFinishedInfo) AnnotationSize() int {
	return int(C.Cronet_RequestFinishedInfo_annotations_size(i.ptr))
}

func (i URLRequestFinishedInfo) AnnotationAt(index int) unsafe.Pointer {
	return unsafe.Pointer(C.Cronet_RequestFinishedInfo_annotations_at(i.ptr, C.uint32_t(index)))
}

// FinishedReason
// Returns the reason why the request finished.
func (i URLRequestFinishedInfo) FinishedReason() URLRequestFinishedInfoFinishedReason {
	return URLRequestFinishedInfoFinishedReason(C.Cronet_RequestFinishedInfo_finished_reason_get(i.ptr))
}

func (i URLRequestFinishedInfo) SetMetrics(metrics Metrics) {
	C.Cronet_RequestFinishedInfo_metrics_set(i.ptr, metrics.ptr)
}

func (i URLRequestFinishedInfo) AddAnnotation(annotation unsafe.Pointer) {
	C.Cronet_RequestFinishedInfo_annotations_add(i.ptr, C.Cronet_RawDataPtr(annotation))
}

func (i URLRequestFinishedInfo) ClearAnnotations() {
	C.Cronet_RequestFinishedInfo_annotations_clear(i.ptr)
}

func (i URLRequestFinishedInfo) SetFinishedReason(reason URLRequestFinishedInfoFinishedReason) {
	C.Cronet_RequestFinishedInfo_finished_reason_set(i.ptr, C.Cronet_RequestFinishedInfo_FINISHED_REASON(reason))
}
