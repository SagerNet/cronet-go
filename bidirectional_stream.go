package cronet

// #cgo LDFLAGS: -lcronet.100.0.4896.60
// #include <stdbool.h>
// #include <stdlib.h>
// #include "cronet_c.h"
// #include "bidirectional_stream_c.h"
// extern void cronetOnStreamReady(bidirectional_stream* stream);
// extern void cronetOnResponseHeadersReceived(bidirectional_stream* stream, bidirectional_stream_header_array* headers, char* negotiated_protocol);
// extern void cronetOnReadCompleted(bidirectional_stream* stream, char* data, int bytes_read);
// extern void cronetOnWriteCompleted(bidirectional_stream* stream, char* data);
// extern void cronetOnResponseTrailersReceived(bidirectional_stream* stream, bidirectional_stream_header_array* trailers);
// extern void cronetOnSucceed(bidirectional_stream* stream);
// extern void cronetOnFailed(bidirectional_stream* stream, int net_error);
// extern void cronetOnCanceled(bidirectional_stream* stream);
import "C"

import (
	"context"
	"io"
	"net"
	"os"
	"reflect"
	"sync"
	"time"
	"unsafe"

	"github.com/sagernet/sing/common"
)

type StreamEngine struct {
	ptr *C.stream_engine
}

func (e *Engine) StreamEngine() *StreamEngine {
	return &StreamEngine{C.Cronet_Engine_GetStreamEngine(e.ptr)}
}

var bidirectionalStreamCallback C.bidirectional_stream_callback

func init() {
	bidirectionalStreamCallback.on_stream_ready = (*[0]byte)(C.cronetOnStreamReady)
	bidirectionalStreamCallback.on_response_headers_received = (*[0]byte)(C.cronetOnResponseHeadersReceived)
	bidirectionalStreamCallback.on_read_completed = (*[0]byte)(C.cronetOnReadCompleted)
	bidirectionalStreamCallback.on_write_completed = (*[0]byte)(C.cronetOnWriteCompleted)
	bidirectionalStreamCallback.on_response_trailers_received = (*[0]byte)(C.cronetOnResponseTrailersReceived)
	bidirectionalStreamCallback.on_succeded = (*[0]byte)(C.cronetOnSucceed)
	bidirectionalStreamCallback.on_failed = (*[0]byte)(C.cronetOnFailed)
	bidirectionalStreamCallback.on_canceled = (*[0]byte)(C.cronetOnCanceled)
}

var streams map[uintptr]*BidirectionalStream

func init() {
	streams = make(map[uintptr]*BidirectionalStream)
}

func instanceOf(stream *C.bidirectional_stream) *BidirectionalStream {
	return streams[uintptr(unsafe.Pointer(stream))]
}

func (e *StreamEngine) CreateStream(ctx context.Context) *BidirectionalStream {
	stream := &BidirectionalStream{
		ctx: ctx,

		done:      make(chan struct{}),
		ready:     make(chan struct{}),
		handshake: make(chan struct{}),
		read:      make(chan int),
		write:     make(chan struct{}),

		ptr: C.bidirectional_stream_create(e.ptr, nil, &bidirectionalStreamCallback),
	}
	streams[uintptr(unsafe.Pointer(stream.ptr))] = stream
	return stream
}

type BidirectionalStream struct {
	ptr    *C.bidirectional_stream
	access sync.Mutex

	ctx  context.Context
	done chan struct{}
	err  error

	ready chan struct{}

	handshake       chan struct{}
	responseHeaders map[string]string

	read  chan int
	write chan struct{}
}

//noinspection GoDeferInLoop
func (s *BidirectionalStream) Start(method string, url string, headers map[string]string, priority int, endOfStream bool) error {
	if s.ptr == nil {
		return os.ErrClosed
	}

	var headerArray C.bidirectional_stream_header_array

	headerLen := len(headers)
	if headerLen > 0 {

		cHeadersPtr := C.malloc(C.ulong(int(C.sizeof_struct_bidirectional_stream_header) * headerLen))
		defer C.free(cHeadersPtr)

		cHeadersHeader := reflect.SliceHeader{
			Data: uintptr(cHeadersPtr),
			Len:  headerLen,
			Cap:  headerLen,
		}

		cHeaders := *(*[]C.bidirectional_stream_header)(unsafe.Pointer(&cHeadersHeader))

		/*
			var cType *C.bidirectional_stream_header
			cType = (*C.bidirectional_stream_header)(cHeadersPtr)
			cHeaders := unsafe.Slice(cType, headerLen)
		*/
		var index int
		for key, value := range headers {
			logger.Trace(key, ":", value)

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
	} else {
		headerArray = C.bidirectional_stream_header_array{
			0, 0, nil,
		}
	}

	cMethod := C.CString(method)
	defer C.free(unsafe.Pointer(cMethod))

	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cURL))

	logger.Trace("start stream ", uintptr(unsafe.Pointer(s.ptr)),
		", url=", url,
		", priority=", priority,
		", method=", method,
		", headers=", headers,
		", endOfStream=", endOfStream,
	)

	result := C.bidirectional_stream_start(s.ptr, cURL, C.int(priority), cMethod, &headerArray, C.bool(endOfStream))
	if result != 0 {
		logger.Warn("start stream failed: ", Errno(result))
		return Errno(result)
	}
	return nil
}

func (s *BidirectionalStream) DisableAutoFlush(disable bool) {
	C.bidirectional_stream_disable_auto_flush(s.ptr, C.bool(disable))
}

func (s *BidirectionalStream) DelayHeadersUntilFlush(delay bool) {
	C.bidirectional_stream_delay_request_headers_until_flush(s.ptr, C.bool(delay))
}

func (s *BidirectionalStream) Handshake() chan<- struct{} {
	return s.handshake
}

func (s *BidirectionalStream) Read(p []byte) (n int, err error) {
	select {
	case <-s.handshake:
		break
	case <-s.done:
		return 0, s.err
	}

	s.access.Lock()

	select {
	case <-s.done:
		return 0, s.err
	default:
	}

	C.bidirectional_stream_read(s.ptr, (*C.char)((unsafe.Pointer)(&p[0])), C.int(len(p)))

	s.access.Unlock()

	select {
	case readN := <-s.read:
		if readN == 0 {
			s.close(io.EOF)
			return 0, io.EOF
		}
		return readN, nil
	case <-s.done:
		return 0, s.err
	}
}

func (s *BidirectionalStream) Write(p []byte) (n int, err error) {
	select {
	case <-s.ready:
		break
	case <-s.done:
		return 0, s.err
	}

	s.access.Lock()

	select {
	case <-s.done:
		return 0, s.err
	default:
	}

	C.bidirectional_stream_write(s.ptr, (*C.char)(unsafe.Pointer(&p[0])), C.int(len(p)), false)

	s.access.Unlock()

	select {
	case <-s.write:
		logger.Trace("ended write")
		return len(p), nil
	case <-s.done:
		return 0, s.err
	}
}

func (s *BidirectionalStream) WriteDirect(p []byte) (n int, err error) {
	select {
	case <-s.ready:
		break
	case <-s.done:
		return 0, s.err
	}

	s.access.Lock()

	select {
	case <-s.done:
		return 0, s.err
	default:
	}

	C.bidirectional_stream_write(s.ptr, (*C.char)(unsafe.Pointer(&p[0])), C.int(len(p)), false)
	s.access.Unlock()
	return len(p), nil
}

func (s *BidirectionalStream) WaitForWriteComplete() error {
	select {
	case <-s.write:
		return nil
	case <-s.done:
		return s.err
	}
}

func (s *BidirectionalStream) Flush() error {
	select {
	case <-s.ready:
		break
	default:
		return os.ErrInvalid
	}
	C.bidirectional_stream_flush(s.ptr)
	return nil
}

//export cronetOnStreamReady
func cronetOnStreamReady(stream *C.bidirectional_stream) {
	logger.Trace("on steam ready")

	instance := instanceOf(stream)
	if instance == nil {
		return
	}
	close(instance.ready)
}

//export cronetOnResponseHeadersReceived
func cronetOnResponseHeadersReceived(stream *C.bidirectional_stream, headers *C.bidirectional_stream_header_array, negotiatedProtocol *C.char) {
	// TODO: add api
	instance := instanceOf(stream)
	if instance == nil {
		return
	}

	close(instance.handshake)

	logger.Trace("on response headers, negotiated_protocol=", C.GoString(negotiatedProtocol))

	var hdrP *C.bidirectional_stream_header
	hdrP = headers.headers

	headersSlice := unsafe.Slice(hdrP, int(headers.count))
	for _, header := range headersSlice {
		key := C.GoString(header.key)
		if len(key) == 0 {
			continue
		}
		value := C.GoString(header.value)
		logger.Trace(key, ": ", value)
	}
}

//export cronetOnReadCompleted
func cronetOnReadCompleted(stream *C.bidirectional_stream, data *C.char, bytesRead C.int) {
	logger.Trace("on read completed")

	instance := instanceOf(stream)
	if instance == nil {
		return
	}

	instance.access.Lock()
	defer instance.access.Unlock()

	if instance.err != nil {
		return
	}

	select {
	case <-instance.done:
	case instance.read <- int(bytesRead):
	}
}

//export cronetOnWriteCompleted
func cronetOnWriteCompleted(stream *C.bidirectional_stream, data *C.char) {
	logger.Trace("on write completed")

	instance := instanceOf(stream)
	if instance == nil {
		return
	}

	instance.access.Lock()
	defer instance.access.Unlock()

	if instance.err != nil {
		return
	}

	select {
	case <-instance.done:
	case instance.write <- struct{}{}:
	}
}

//export cronetOnResponseTrailersReceived
func cronetOnResponseTrailersReceived(stream *C.bidirectional_stream, trailers *C.bidirectional_stream_header_array) {
	// TODO: add api

	logger.Trace("on response trailers received")

	var hdrP *C.bidirectional_stream_header
	hdrP = trailers.headers

	headersSlice := unsafe.Slice(hdrP, int(trailers.count))
	for _, header := range headersSlice {
		key := C.GoString(header.key)
		if len(key) == 0 {
			continue
		}
		value := C.GoString(header.value)
		logger.Trace(key, ": ", value)
	}
}

//export cronetOnSucceed
func cronetOnSucceed(stream *C.bidirectional_stream) {
	logger.Trace("on succeed")

	instance := instanceOf(stream)
	if instance == nil {
		return
	}

	instance.close(io.EOF)
}

//export cronetOnFailed
func cronetOnFailed(stream *C.bidirectional_stream, netError C.int) {
	logger.Trace("on failed, error=", Errno(netError))

	instance := instanceOf(stream)
	if instance == nil {
		return
	}

	instance.close(Errno(netError))
}

//export cronetOnCanceled
func cronetOnCanceled(stream *C.bidirectional_stream) {
	logger.Trace("on canceled")

	instance := instanceOf(stream)
	if instance == nil {
		return
	}

	instance.close(context.Canceled)
}

func (s *BidirectionalStream) close(err error) {
	if s.err != nil {
		return
	}

	s.access.Lock()
	defer s.access.Unlock()

	if s.err != nil {
		return
	}

	s.err = err
	close(s.done)

	delete(streams, uintptr(unsafe.Pointer(s.ptr)))
	C.bidirectional_stream_destroy(s.ptr)
	s.ptr = nil
}

func (s *BidirectionalStream) Done() <-chan struct{} {
	return s.done
}

func (s *BidirectionalStream) Err() error {
	return s.err
}

func (s *BidirectionalStream) Deadline() (deadline time.Time, ok bool) {
	return s.ctx.Deadline()
}

func (s *BidirectionalStream) Value(key any) any {
	return s.ctx.Value(key)
}

func (s *BidirectionalStream) Close() error {
	s.access.Lock()
	defer s.access.Unlock()
	if s.ptr == nil {
		return os.ErrClosed
	}
	C.bidirectional_stream_cancel(s.ptr)
	return nil
}

func (s *BidirectionalStream) LocalAddr() net.Addr {
	return &common.DummyAddr{}
}

func (s *BidirectionalStream) RemoteAddr() net.Addr {
	return &common.DummyAddr{}
}

func (s *BidirectionalStream) SetDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (s *BidirectionalStream) SetReadDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (s *BidirectionalStream) SetWriteDeadline(t time.Time) error {
	return os.ErrInvalid
}
