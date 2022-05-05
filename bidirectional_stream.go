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

type StreamEngine struct {
	ptr *C.stream_engine
}

func (e Engine) StreamEngine() StreamEngine {
	return StreamEngine{C.Cronet_Engine_GetStreamEngine(e.ptr)}
}

type BidirectionalStreamCallback interface {
	OnStreamReady(stream BidirectionalStream)
	OnResponseHeadersReceived(stream BidirectionalStream, headers map[string]string, negotiatedProtocol string)
	OnReadCompleted(stream BidirectionalStream, bytesRead int)
	OnWriteCompleted(stream BidirectionalStream)
	OnResponseTrailersReceived(stream BidirectionalStream, trailers map[string]string)
	OnSucceed(stream BidirectionalStream)
	OnFailed(stream BidirectionalStream, netError int)
	OnCanceled(stream BidirectionalStream)
}

type BidirectionalStream struct {
	ptr *C.bidirectional_stream
}

func (e StreamEngine) CreateStream(callback BidirectionalStreamCallback) BidirectionalStream {
	ptr := C.bidirectional_stream_create(e.ptr, nil, &bidirectionalStreamCallback)
	bidirectionalStreamAccess.Lock()
	bidirectionalStreamMap[uintptr(unsafe.Pointer(ptr))] = callback
	bidirectionalStreamAccess.Unlock()
	return BidirectionalStream{ptr}
}

// Destroy destroys stream object. Destroy could be called from any thread, including
// network thread, but is posted, so |stream| is valid until calling task is
// complete.
func (c BidirectionalStream) Destroy() bool {
	bidirectionalStreamAccess.Lock()
	delete(bidirectionalStreamMap, uintptr(unsafe.Pointer(c.ptr)))
	bidirectionalStreamAccess.Unlock()
	return C.bidirectional_stream_destroy(c.ptr) == 0
}

// DisableAutoFlush disables or enables auto flush. By default, data is flushed after
// every Write(). If the auto flush is disabled,
// the client should explicitly call Flush() to flush
// the data.
func (c BidirectionalStream) DisableAutoFlush(disable bool) {
	C.bidirectional_stream_disable_auto_flush(c.ptr, C.bool(disable))
}

// DelayRequestHeadersUntilFlush delays sending request headers until Flush()
// is called. This flag is currently only respected when QUIC is negotiated.
// When true, QUIC will send request header frame along with data frame(s)
// as a single packet when possible.
func (c BidirectionalStream) DelayRequestHeadersUntilFlush(delay bool) {
	C.bidirectional_stream_delay_request_headers_until_flush(c.ptr, C.bool(delay))
}

// Start starts the stream by sending request to |url| using |method| and |headers|.
// If |endOfStream| is true, then no data is expected to be written. The
// |method| is HTTP verb.
//noinspection GoDeferInLoop
func (c BidirectionalStream) Start(method string, url string, headers map[string]string, priority int, endOfStream bool) bool {
	var headerArray C.bidirectional_stream_header_array
	headerLen := len(headers)
	if headerLen > 0 {
		cHeadersPtr := C.malloc(C.ulong(int(C.sizeof_struct_bidirectional_stream_header) * headerLen))
		defer C.free(cHeadersPtr)
		var cType *C.bidirectional_stream_header
		cType = (*C.bidirectional_stream_header)(cHeadersPtr)
		cHeaders := unsafe.Slice(cType, headerLen)
		var index int
		for key, value := range headers {
			cKey := C.CString(key)
			defer C.free(unsafe.Pointer(cKey))
			cValue := C.CString(value)
			defer C.free(unsafe.Pointer(cValue))
			cHeaders[index].key = cKey
			cHeaders[index].value = cValue
			index++
		}
		headerArray = C.bidirectional_stream_header_array{
			C.ulong(headerLen), C.ulong(headerLen), &cHeaders[0],
		}
	}

	cMethod := C.CString(method)
	defer C.free(unsafe.Pointer(cMethod))

	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cURL))

	return C.bidirectional_stream_start(c.ptr, cURL, C.int(priority), cMethod, &headerArray, C.bool(endOfStream)) == 0
}

// Read reads response data into |buffer|. Must only be called
// at most once in response to each invocation of the
// OnStreamReady()/OnResponseHeaderReceived() and OnReadCompleted()
// methods of the BidirectionalStreamCallback.
// Each call will result in an invocation of the callback's
// OnReadCompleted() method if data is read, or its OnFailed() method if
// there's an error. The callback's OnSucceed() method is also invoked if
// there is no more data to read and |end_of_stream| was previously sent.
func (c BidirectionalStream) Read(buffer []byte) int {
	return int(C.bidirectional_stream_read(c.ptr, (*C.char)((unsafe.Pointer)(&buffer[0])), C.int(len(buffer))))
}

// Write Writes request data from |buffer| If auto flush is
// disabled, data will be sent only after Flush() is
// called.
// Each call will result in an invocation the callback's OnWriteCompleted()
// method if data is sent, or its OnFailed() method if there's an error.
// The callback's OnSucceed() method is also invoked if |endOfStream| is
// set and all response data has been read.
func (c BidirectionalStream) Write(buffer []byte, endOfStream bool) int {
	return int(C.bidirectional_stream_write(c.ptr, (*C.char)(unsafe.Pointer(&buffer[0])), C.int(len(buffer)), C.bool(endOfStream)))
}

// Flush Flushes pending writes. This method should not be called before invocation of
// OnStreamReady() method of the BidirectionalStreamCallback.
// For each previously called Write()
// a corresponding OnWriteCompleted() callback will be invoked when the buffer
// is sent.BidirectionalStream
func (c BidirectionalStream) Flush() {
	C.bidirectional_stream_flush(c.ptr)
}

// Cancel cancels the stream. Can be called at any time after
// Start(). The OnCanceled() method of
// BidirectionalStreamCallback will be invoked when cancellation
// is complete and no further callback methods will be invoked. If the
// stream has completed or has not started, calling
// Cancel() has no effect and OnCanceled() will not
// be invoked. At most one callback method may be invoked after
// Cancel() has completed.
func (c BidirectionalStream) Cancel() {
	C.bidirectional_stream_cancel(c.ptr)
}

/*// IsDone returns true if the stream was successfully started and is now done
// (succeeded, canceled, or failed).
// returns false if the stream is not yet started or is in progress.
func (c BidirectionalStream) IsDone() bool {
	return bool(C.bidirectional_stream_is_done(c.ptr))
}*/

// wire

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
	callback.OnSucceed(BidirectionalStream{stream})
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
