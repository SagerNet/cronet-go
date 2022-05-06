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
	inner            bidirectionalHandler
	readWaitHeaders  bool
	writeWaitHeaders bool
}

func (e StreamEngine) CreateConn(readWaitHeaders bool, writeWaitHeaders bool) *BidirectionalConn {
	conn := &BidirectionalConn{
		readWaitHeaders:  readWaitHeaders,
		writeWaitHeaders: writeWaitHeaders,
	}
	conn.inner = bidirectionalHandler{
		close:     make(chan struct{}),
		done:      make(chan struct{}),
		ready:     make(chan struct{}),
		handshake: make(chan struct{}),
		read:      make(chan int),
		write:     make(chan struct{}),
	}
	conn.stream = e.CreateStream(&conn.inner)
	return conn
}

func (c *BidirectionalConn) Start(method string, url string, headers map[string]string, priority int, endOfStream bool) error {
	c.inner.access.Lock()
	defer c.inner.access.Unlock()
	select {
	case <-c.inner.close:
		return net.ErrClosed
	case <-c.inner.done:
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
	case <-c.inner.close:
		return 0, net.ErrClosed
	case <-c.inner.done:
		return 0, net.ErrClosed
	default:
	}

	if c.readWaitHeaders {
		select {
		case <-c.inner.handshake:
			break
		case <-c.inner.done:
			return 0, c.inner.err
		}
	} else {
		select {
		case <-c.inner.ready:
			break
		case <-c.inner.done:
			return 0, c.inner.err
		}
	}

	c.inner.access.Lock()

	select {
	case <-c.inner.close:
		return 0, net.ErrClosed
	case <-c.inner.done:
		return 0, net.ErrClosed
	default:
	}

	c.stream.Read(p)
	c.inner.access.Unlock()

	select {
	case bytesRead := <-c.inner.read:
		return bytesRead, nil
	case <-c.inner.done:
		return 0, c.inner.err
	}
}

// Write implements io.Writer
func (c *BidirectionalConn) Write(p []byte) (n int, err error) {
	select {
	case <-c.inner.close:
		return 0, net.ErrClosed
	case <-c.inner.done:
		return 0, net.ErrClosed
	default:
	}

	if c.writeWaitHeaders {
		select {
		case <-c.inner.handshake:
			break
		case <-c.inner.done:
			return 0, c.inner.err
		}
	} else {
		select {
		case <-c.inner.ready:
			break
		case <-c.inner.done:
			return 0, c.inner.err
		}
	}

	c.inner.access.Lock()

	select {
	case <-c.inner.close:
		return 0, net.ErrClosed
	case <-c.inner.done:
		return 0, net.ErrClosed
	default:
	}

	c.stream.Write(p, false)
	c.inner.access.Unlock()

	select {
	case <-c.inner.write:
		return len(p), nil
	case <-c.inner.done:
		return 0, c.inner.err
	}
}

// Done implements context.Context
func (c *BidirectionalConn) Done() <-chan struct{} {
	return c.inner.done
}

// Err implements context.Context
func (c *BidirectionalConn) Err() error {
	return c.inner.err
}

// Close implements io.Closer
func (c *BidirectionalConn) Close() error {
	c.inner.access.Lock()
	defer c.inner.access.Unlock()

	select {
	case <-c.inner.close:
		return net.ErrClosed
	case <-c.inner.done:
		return net.ErrClosed
	default:
	}

	close(c.inner.close)
	c.stream.Cancel()
	return nil
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

type bidirectionalHandler struct {
	access    sync.Mutex
	close     chan struct{}
	done      chan struct{}
	err       error
	ready     chan struct{}
	handshake chan struct{}
	read      chan int
	write     chan struct{}
}

func (c *bidirectionalHandler) OnStreamReady(stream BidirectionalStream) {
	close(c.ready)
}

func (c *bidirectionalHandler) OnResponseHeadersReceived(stream BidirectionalStream, headers map[string]string, negotiatedProtocol string) {
	close(c.handshake)
}

func (c *bidirectionalHandler) OnReadCompleted(stream BidirectionalStream, bytesRead int) {
	c.access.Lock()

	if c.err != nil {
		c.access.Unlock()
		return
	}

	if bytesRead == 0 {
		c.access.Unlock()
		c.Close(stream, io.EOF)
		return
	}

	select {
	case <-c.close:
	case <-c.done:
	case c.read <- bytesRead:
	}

	c.access.Unlock()
}

func (c *bidirectionalHandler) OnWriteCompleted(stream BidirectionalStream) {
	c.access.Lock()
	defer c.access.Unlock()

	if c.err != nil {
		return
	}

	select {
	case <-c.close:
	case <-c.done:
	case c.write <- struct{}{}:
	}
}

func (c *bidirectionalHandler) OnResponseTrailersReceived(stream BidirectionalStream, trailers map[string]string) {
}

func (c *bidirectionalHandler) OnSucceed(stream BidirectionalStream) {
	c.Close(stream, io.EOF)
}

func (c *bidirectionalHandler) OnFailed(stream BidirectionalStream, netError int) {
	c.Close(stream, errors.New("network error "+strconv.Itoa(netError)))
}

func (c *bidirectionalHandler) OnCanceled(stream BidirectionalStream) {
	c.Close(stream, context.Canceled)
}

func (c *bidirectionalHandler) Close(stream BidirectionalStream, err error) {
	c.access.Lock()
	defer c.access.Unlock()

	select {
	case <-c.done:
		return
	default:
	}

	c.err = err
	close(c.done)

	stream.Destroy()
}
