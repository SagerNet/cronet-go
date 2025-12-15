package cronet

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

// BidirectionalConn is a wrapper from BidirectionalStream to net.Conn
type BidirectionalConn struct {
	stream           BidirectionalStream
	readWaitHeaders  bool
	writeWaitHeaders bool
	access           sync.Mutex
	close            chan struct{}
	done             chan struct{}
	err              error
	ready            chan struct{}
	handshake        chan struct{}
	read             chan int
	write            chan struct{}
	headers          map[string]string

	// readSemaphore/writeSemaphore ensure that only one Read/Write operation is
	// in-flight at a time. This is required for Cronet and prevents reuse of
	// internal buffers while Cronet still holds pointers to them.
	readSemaphore  chan struct{}
	writeSemaphore chan struct{}

	// Cronet holds pointers to the Read/Write buffers asynchronously until the
	// corresponding callback fires. Never pass caller-provided buffers directly,
	// as they might reside on the Go stack and be moved during stack growth.
	readBuffer  []byte
	writeBuffer []byte

	// Buffer safety: when Read/Write return due to close/done, Cronet may
	// still hold the buffer. These channels are closed by callbacks to signal
	// it's safe to return. sync.Once ensures close happens exactly once.
	readDone      chan struct{}
	writeDone     chan struct{}
	readDoneOnce  sync.Once
	writeDoneOnce sync.Once
}

func (e StreamEngine) CreateConn(readWaitHeaders bool, writeWaitHeaders bool) *BidirectionalConn {
	conn := &BidirectionalConn{
		readWaitHeaders:  readWaitHeaders,
		writeWaitHeaders: writeWaitHeaders,
		close:            make(chan struct{}),
		done:             make(chan struct{}),
		ready:            make(chan struct{}),
		handshake:        make(chan struct{}),
		read:             make(chan int),
		write:            make(chan struct{}),
		readSemaphore:    make(chan struct{}, 1),
		writeSemaphore:   make(chan struct{}, 1),
		readDone:         make(chan struct{}, 1),
		writeDone:        make(chan struct{}, 1),
	}
	conn.readSemaphore <- struct{}{}
	conn.writeSemaphore <- struct{}{}
	conn.stream = e.CreateStream(&bidirectionalHandler{BidirectionalConn: conn})
	return conn
}

func (c *BidirectionalConn) Start(method string, url string, headers map[string]string, priority int, endOfStream bool) error {
	c.access.Lock()
	defer c.access.Unlock()
	select {
	case <-c.close:
		return net.ErrClosed
	case <-c.done:
		return net.ErrClosed
	default:
	}
	if !c.stream.Start(method, url, headers, priority, endOfStream) {
		return os.ErrInvalid
	}
	return nil
}

// Read implements io.Reader
func (c *BidirectionalConn) Read(p []byte) (n int, err error) {
	select {
	case <-c.close:
		return 0, net.ErrClosed
	case <-c.done:
		return 0, net.ErrClosed
	default:
	}
	if len(p) == 0 {
		return 0, nil
	}

	select {
	case <-c.close:
		return 0, net.ErrClosed
	case <-c.done:
		return 0, net.ErrClosed
	case <-c.readSemaphore:
	}
	defer func() { c.readSemaphore <- struct{}{} }()

	if c.readWaitHeaders {
		select {
		case <-c.handshake:
			break
		case <-c.done:
			return 0, c.err
		}
	} else {
		select {
		case <-c.ready:
			break
		case <-c.done:
			return 0, c.err
		}
	}

	c.access.Lock()

	select {
	case <-c.close:
		c.access.Unlock()
		return 0, net.ErrClosed
	case <-c.done:
		c.access.Unlock()
		return 0, net.ErrClosed
	default:
	}

	if len(c.readBuffer) < len(p) {
		c.readBuffer = make([]byte, len(p))
	}
	readBuffer := c.readBuffer[:len(p)]
	c.stream.Read(readBuffer)
	c.access.Unlock()

	select {
	case bytesRead := <-c.read:
		if bytesRead > len(p) {
			bytesRead = len(p)
		}
		if bytesRead > 0 {
			copy(p, readBuffer[:bytesRead])
		}
		return bytesRead, nil
	case <-c.done:
		// Wait for Cronet to finish using the buffer before returning.
		// Callbacks will close readDone when done.
		<-c.readDone
		return 0, c.err
	case <-c.close:
		// Close() was called. Wait for OnCanceled to signal buffer safety.
		<-c.readDone
		return 0, net.ErrClosed
	}
}

// Write implements io.Writer
func (c *BidirectionalConn) Write(p []byte) (n int, err error) {
	select {
	case <-c.close:
		return 0, net.ErrClosed
	case <-c.done:
		return 0, net.ErrClosed
	default:
	}
	if len(p) == 0 {
		return 0, nil
	}

	select {
	case <-c.close:
		return 0, net.ErrClosed
	case <-c.done:
		return 0, net.ErrClosed
	case <-c.writeSemaphore:
	}
	defer func() { c.writeSemaphore <- struct{}{} }()

	if c.writeWaitHeaders {
		select {
		case <-c.handshake:
			break
		case <-c.done:
			return 0, c.err
		}
	} else {
		select {
		case <-c.ready:
			break
		case <-c.done:
			return 0, c.err
		}
	}

	c.access.Lock()

	select {
	case <-c.close:
		c.access.Unlock()
		return 0, net.ErrClosed
	case <-c.done:
		c.access.Unlock()
		return 0, net.ErrClosed
	default:
	}

	if len(c.writeBuffer) < len(p) {
		c.writeBuffer = make([]byte, len(p))
	}
	writeBuffer := c.writeBuffer[:len(p)]
	copy(writeBuffer, p)
	c.stream.Write(writeBuffer, false)
	c.access.Unlock()

	select {
	case <-c.write:
		return len(p), nil
	case <-c.done:
		// Wait for Cronet to finish using the buffer before returning.
		// Callbacks will close writeDone when done.
		<-c.writeDone
		return 0, c.err
	case <-c.close:
		// Close() was called. Wait for OnCanceled to signal buffer safety.
		<-c.writeDone
		return 0, net.ErrClosed
	}
}

// Done implements context.Context
func (c *BidirectionalConn) Done() <-chan struct{} {
	return c.done
}

// Err implements context.Context
func (c *BidirectionalConn) Err() error {
	return c.err
}

// Close implements io.Closer
func (c *BidirectionalConn) Close() error {
	c.access.Lock()

	select {
	case <-c.close:
		c.access.Unlock()
		return net.ErrClosed
	case <-c.done:
		c.access.Unlock()
		return net.ErrClosed
	default:
	}

	close(c.close)
	c.access.Unlock()

	// Cancel must be called without holding the mutex to avoid deadlock
	// with callbacks (OnCanceled -> bidirectionalHandler.Close -> mutex)
	c.stream.Cancel()
	return nil
}

func (c *BidirectionalConn) signalReadDone() {
	c.readDoneOnce.Do(func() { close(c.readDone) })
}

func (c *BidirectionalConn) signalWriteDone() {
	c.writeDoneOnce.Do(func() { close(c.writeDone) })
}

// LocalAddr implements net.Conn
func (c *BidirectionalConn) LocalAddr() net.Addr {
	return nil
}

// RemoteAddr implements net.Conn
func (c *BidirectionalConn) RemoteAddr() net.Addr {
	return nil
}

// SetDeadline implements net.Conn
func (c *BidirectionalConn) SetDeadline(t time.Time) error {
	return os.ErrInvalid
}

// SetReadDeadline implements net.Conn
func (c *BidirectionalConn) SetReadDeadline(t time.Time) error {
	return os.ErrInvalid
}

// SetWriteDeadline implements net.Conn
func (c *BidirectionalConn) SetWriteDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *BidirectionalConn) WaitForHeaders() (map[string]string, error) {
	select {
	case <-c.close:
		return nil, net.ErrClosed
	case <-c.done:
		return nil, net.ErrClosed
	default:
	}

	select {
	case <-c.handshake:
		return c.headers, nil
	case <-c.done:
		return nil, c.err
	}
}

func (c *BidirectionalConn) WaitForHeadersContext(ctx context.Context) (map[string]string, error) {
	select {
	case <-c.close:
		return nil, net.ErrClosed
	case <-c.done:
		return nil, net.ErrClosed
	default:
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.handshake:
		return c.headers, nil
	case <-c.done:
		return nil, c.err
	case <-c.close:
		return nil, net.ErrClosed
	}
}

type bidirectionalHandler struct {
	*BidirectionalConn
	readyOnce     sync.Once
	handshakeOnce sync.Once
	doneOnce      sync.Once
}

func (c *bidirectionalHandler) OnStreamReady(stream BidirectionalStream) {
	c.readyOnce.Do(func() { close(c.ready) })
}

func (c *bidirectionalHandler) OnResponseHeadersReceived(stream BidirectionalStream, headers map[string]string, negotiatedProtocol string) {
	c.headers = headers
	c.handshakeOnce.Do(func() { close(c.handshake) })
}

func (c *bidirectionalHandler) OnReadCompleted(stream BidirectionalStream, bytesRead int) {
	c.access.Lock()

	if c.err != nil {
		c.access.Unlock()
		c.signalReadDone()
		return
	}

	if bytesRead == 0 {
		c.access.Unlock()
		c.signalReadDone()
		c.signalWriteDone()
		c.Close(stream, io.EOF)
		return
	}

	c.access.Unlock()

	select {
	case <-c.close:
		c.signalReadDone()
	case <-c.done:
		c.signalReadDone()
	case c.read <- bytesRead:
	}
}

func (c *bidirectionalHandler) OnWriteCompleted(stream BidirectionalStream) {
	c.access.Lock()

	if c.err != nil {
		c.access.Unlock()
		c.signalWriteDone()
		return
	}

	c.access.Unlock()

	select {
	case <-c.close:
		c.signalWriteDone()
	case <-c.done:
		c.signalWriteDone()
	case c.write <- struct{}{}:
	}
}

func (c *bidirectionalHandler) OnResponseTrailersReceived(stream BidirectionalStream, trailers map[string]string) {
}

func (c *bidirectionalHandler) OnSucceeded(stream BidirectionalStream) {
	c.signalReadDone()
	c.signalWriteDone()
	c.Close(stream, io.EOF)
}

func (c *bidirectionalHandler) OnFailed(stream BidirectionalStream, netError int) {
	c.signalReadDone()
	c.signalWriteDone()
	c.Close(stream, errors.New("network error "+strconv.Itoa(netError)))
}

func (c *bidirectionalHandler) OnCanceled(stream BidirectionalStream) {
	c.signalReadDone()
	c.signalWriteDone()
	c.Close(stream, context.Canceled)
}

func (c *bidirectionalHandler) Close(stream BidirectionalStream, err error) {
	c.doneOnce.Do(func() {
		c.access.Lock()
		c.err = err
		close(c.done)
		c.access.Unlock()
		stream.Destroy()
	})
}
