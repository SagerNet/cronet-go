package cronet

// #include <stdbool.h>
// #include <stdlib.h>
// #include <cronet_c.h>
// #include <bidirectional_stream_c.h>
// extern void cronetBidirectionalStreamOnStreamReady(bidirectional_stream* stream);
// extern void cronetBidirectionalStreamOnResponseHeadersReceived(bidirectional_stream* stream, bidirectional_stream_header_array* headers, char* negotiated_protocol);
// extern void cronetBidirectionalStreamOnReadCompleted(bidirectional_stream* stream, char* data, int bytes_read);
// extern void cronetBidirectionalStreamOnWriteCompleted(bidirectional_stream* stream, char* data);
// extern void cronetBidirectionalStreamOnResponseTrailersReceived(bidirectional_stream* stream, bidirectional_stream_header_array* trailers);
// extern void cronetBidirectionalStreamOnSucceed(bidirectional_stream* stream);
// extern void cronetBidirectionalStreamOnFailed(bidirectional_stream* stream, int net_error);
// extern void cronetBidirectionalStreamOnCanceled(bidirectional_stream* stream);
import "C"

import (
	"sync"
	"unsafe"
)

var (
	bidirectionalStreamAccess   sync.RWMutex
	bidirectionalStreamMap      map[uintptr]BidirectionalStreamCallback
	bidirectionalStreamCallback C.bidirectional_stream_callback
)

func init() {
	bidirectionalStreamMap = make(map[uintptr]BidirectionalStreamCallback)
	bidirectionalStreamCallback.on_stream_ready = (*[0]byte)(C.cronetBidirectionalStreamOnStreamReady)
	bidirectionalStreamCallback.on_response_headers_received = (*[0]byte)(C.cronetBidirectionalStreamOnResponseHeadersReceived)
	bidirectionalStreamCallback.on_read_completed = (*[0]byte)(C.cronetBidirectionalStreamOnReadCompleted)
	bidirectionalStreamCallback.on_write_completed = (*[0]byte)(C.cronetBidirectionalStreamOnWriteCompleted)
	bidirectionalStreamCallback.on_response_trailers_received = (*[0]byte)(C.cronetBidirectionalStreamOnResponseTrailersReceived)
	bidirectionalStreamCallback.on_succeded = (*[0]byte)(C.cronetBidirectionalStreamOnSucceed)
	bidirectionalStreamCallback.on_failed = (*[0]byte)(C.cronetBidirectionalStreamOnFailed)
	bidirectionalStreamCallback.on_canceled = (*[0]byte)(C.cronetBidirectionalStreamOnCanceled)
}

func instanceOfBidirectionalStream(stream *C.bidirectional_stream) BidirectionalStreamCallback {
	bidirectionalStreamAccess.RLock()
	defer bidirectionalStreamAccess.RUnlock()
	return bidirectionalStreamMap[uintptr(unsafe.Pointer(stream))]
}

//export cronetBidirectionalStreamOnStreamReady
func cronetBidirectionalStreamOnStreamReady(stream *C.bidirectional_stream) {
	callback := instanceOfBidirectionalStream(stream)
	if callback == nil {
		return
	}
	callback.OnStreamReady(BidirectionalStream{stream})
}

//export cronetBidirectionalStreamOnResponseHeadersReceived
func cronetBidirectionalStreamOnResponseHeadersReceived(stream *C.bidirectional_stream, headers *C.bidirectional_stream_header_array, negotiatedProtocol *C.char) {
	callback := instanceOfBidirectionalStream(stream)
	if callback == nil {
		return
	}
	headerMap := make(map[string]string, int(headers.count))
	var hdrP *C.bidirectional_stream_header
	hdrP = headers.headers
	headersSlice := unsafe.Slice(hdrP, int(headers.count))
	for _, header := range headersSlice {
		key := C.GoString(header.key)
		if len(key) == 0 {
			continue
		}
		headerMap[key] = C.GoString(header.value)
	}
	callback.OnResponseHeadersReceived(BidirectionalStream{stream}, headerMap, C.GoString(negotiatedProtocol))
}

//export cronetBidirectionalStreamOnReadCompleted
func cronetBidirectionalStreamOnReadCompleted(stream *C.bidirectional_stream, data *C.char, bytesRead C.int) {
	callback := instanceOfBidirectionalStream(stream)
	if callback == nil {
		return
	}
	callback.OnReadCompleted(BidirectionalStream{stream}, int(bytesRead))
}

//export cronetBidirectionalStreamOnWriteCompleted
func cronetBidirectionalStreamOnWriteCompleted(stream *C.bidirectional_stream, data *C.char) {
	callback := instanceOfBidirectionalStream(stream)
	if callback == nil {
		return
	}
	callback.OnWriteCompleted(BidirectionalStream{stream})
}

//export cronetBidirectionalStreamOnResponseTrailersReceived
func cronetBidirectionalStreamOnResponseTrailersReceived(stream *C.bidirectional_stream, trailers *C.bidirectional_stream_header_array) {
	callback := instanceOfBidirectionalStream(stream)
	if callback == nil {
		return
	}
	trailersMap := make(map[string]string, int(trailers.count))
	var hdrP *C.bidirectional_stream_header
	hdrP = trailers.headers
	headersSlice := unsafe.Slice(hdrP, int(trailers.count))
	for _, header := range headersSlice {
		key := C.GoString(header.key)
		if len(key) == 0 {
			continue
		}
		trailersMap[key] = C.GoString(header.value)
	}
	callback.OnResponseTrailersReceived(BidirectionalStream{stream}, trailersMap)
}

//export cronetBidirectionalStreamOnSucceed
func cronetBidirectionalStreamOnSucceed(stream *C.bidirectional_stream) {
	callback := instanceOfBidirectionalStream(stream)
	if callback == nil {
		return
	}
	callback.OnSucceeded(BidirectionalStream{stream})
}

//export cronetBidirectionalStreamOnFailed
func cronetBidirectionalStreamOnFailed(stream *C.bidirectional_stream, netError C.int) {
	callback := instanceOfBidirectionalStream(stream)
	if callback == nil {
		return
	}
	callback.OnFailed(BidirectionalStream{stream}, int(netError))
}

//export cronetBidirectionalStreamOnCanceled
func cronetBidirectionalStreamOnCanceled(stream *C.bidirectional_stream) {
	callback := instanceOfBidirectionalStream(stream)
	if callback == nil {
		return
	}
	callback.OnCanceled(BidirectionalStream{stream})
}
