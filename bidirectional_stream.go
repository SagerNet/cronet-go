package cronet

// #include <stdbool.h>
// #include <stdlib.h>
// #include <cronet_c.h>
// #include <bidirectional_stream_c.h>
import "C"

import (
	"unsafe"
)

// StreamEngine
// Opaque object representing a Bidirectional stream creating engine. Created
// and configured outside of this API to facilitate sharing with other
// components
type StreamEngine struct {
	ptr *C.stream_engine
}

func (e Engine) StreamEngine() StreamEngine {
	return StreamEngine{C.Cronet_Engine_GetStreamEngine(e.ptr)}
}

// BidirectionalStream
// Opaque object representing Bidirectional Stream
type BidirectionalStream struct {
	ptr *C.bidirectional_stream
}

// BidirectionalStreamCallback
// Set of callbacks used to receive callbacks from bidirectional stream.
type BidirectionalStreamCallback interface {
	// OnStreamReady
	// Invoked when the stream is ready for reading and writing.
	// Consumer may call BidirectionalStream.Read() to start reading data.
	// Consumer may call BidirectionalStream.Write() to start writing
	// data.
	OnStreamReady(stream BidirectionalStream)

	// OnResponseHeadersReceived
	// Invoked when initial response headers are received.
	// Consumer must call BidirectionalStream.Read() to start reading.
	// Consumer may call BidirectionalStream.Write() to start writing or
	// close the stream. Contents of |headers| is valid for duration of the call.
	///
	OnResponseHeadersReceived(stream BidirectionalStream, headers map[string]string, negotiatedProtocol string)

	// OnReadCompleted
	// Invoked when data is read into the buffer passed to
	// BidirectionalStream.Read(). Only part of the buffer may be
	// populated. To continue reading, call BidirectionalStream.Read().
	// It may be invoked after on_response_trailers_received()}, if there was
	// pending read data before trailers were received.
	//
	// If |bytesRead| is 0, it means the remote side has signaled that it will
	// send no more data; future calls to BidirectionalStream.Read()
	// will result in the OnReadCompleted() callback or OnSucceeded() callback if
	// BidirectionalStream.Write() was invoked with endOfStream set to
	// true.
	OnReadCompleted(stream BidirectionalStream, bytesRead int)

	// OnWriteCompleted
	// Invoked when all data passed to BidirectionalStream.Write() is
	// sent. To continue writing, call BidirectionalStream.Write().
	OnWriteCompleted(stream BidirectionalStream)

	// OnResponseTrailersReceived
	// Invoked when trailers are received before closing the stream. Only invoked
	// when server sends trailers, which it may not. May be invoked while there is
	// read data remaining in local buffer. Contents of |trailers| is valid for
	// duration of the call.
	OnResponseTrailersReceived(stream BidirectionalStream, trailers map[string]string)

	// OnSucceeded
	// Invoked when there is no data to be read or written and the stream is
	// closed successfully remotely and locally. Once invoked, no further callback
	// methods will be invoked.
	OnSucceeded(stream BidirectionalStream)

	// OnFailed
	// Invoked if the stream failed for any reason after
	// BidirectionalStream.Start(). HTTP/2 error codes are
	// mapped to chrome net error codes. Once invoked, no further callback methods
	// will be invoked.
	OnFailed(stream BidirectionalStream, netError int)

	// OnCanceled
	// Invoked if the stream was canceled via
	// BidirectionalStream.Cancel(). Once invoked, no further callback
	// methods will be invoked.
	OnCanceled(stream BidirectionalStream)
}

// CreateStream
// Creates a new stream object that uses |engine| and |callback|. All stream
// tasks are performed asynchronously on the |engine| network thread. |callback|
// methods are invoked synchronously on the |engine| network thread, but must
// not run tasks on the current thread to prevent blocking networking operations
// and causing exceptions during shutdown. The |annotation| is stored in
// bidirectional stream for arbitrary use by application.
//
// Returned |bidirectional_stream*| is owned by the caller, and must be
// destroyed using |bidirectional_stream_destroy|.
//
// Both |calback| and |engine| must remain valid until stream is destroyed.
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
// noinspection GoDeferInLoop
func (c BidirectionalStream) Start(method string, url string, headers map[string]string, priority int, endOfStream bool) bool {
	var headerArray C.bidirectional_stream_header_array
	headerLen := len(headers)
	if headerLen > 0 {
		cHeadersPtr := C.malloc(C.size_t(int(C.sizeof_struct_bidirectional_stream_header) * headerLen))
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
			C.size_t(headerLen), C.size_t(headerLen), &cHeaders[0],
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
// there's an error. The callback's OnSucceeded() method is also invoked if
// there is no more data to read and |end_of_stream| was previously sent.
func (c BidirectionalStream) Read(buffer []byte) int {
	return int(C.bidirectional_stream_read(c.ptr, (*C.char)((unsafe.Pointer)(&buffer[0])), C.int(len(buffer))))
}

// Write Writes request data from |buffer| If auto flush is
// disabled, data will be sent only after Flush() is
// called.
// Each call will result in an invocation the callback's BidirectionalStreamCallback.OnWriteCompleted()
// method if data is sent, or its BidirectionalStreamCallback.OnFailed() method if there's an error.
// The callback's BidirectionalStreamCallback.OnSucceeded() method is also invoked if |endOfStream| is
// set and all response data has been read.
func (c BidirectionalStream) Write(buffer []byte, endOfStream bool) int {
	return int(C.bidirectional_stream_write(c.ptr, (*C.char)(unsafe.Pointer(&buffer[0])), C.int(len(buffer)), C.bool(endOfStream)))
}

// Flush Flushes pending writes. This method should not be called before invocation of
// BidirectionalStreamCallback.OnStreamReady() method.
// For each previously called Write()
// a corresponding OnWriteCompleted() callback will be invoked when the buffer
// is sent.BidirectionalStream
func (c BidirectionalStream) Flush() {
	C.bidirectional_stream_flush(c.ptr)
}

// Cancel cancels the stream. Can be called at any time after
// Start(). The BidirectionalStreamCallback.OnCanceled() method will be invoked when cancellation
// is complete and no further callback methods will be invoked. If the
// stream has completed or has not started, calling
// Cancel() has no effect and BidirectionalStreamCallback.OnCanceled() will not
// be invoked. At most one callback method may be invoked after
// Cancel() has completed.
func (c BidirectionalStream) Cancel() {
	C.bidirectional_stream_cancel(c.ptr)
}
