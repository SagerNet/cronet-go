package cronet

import (
	"context"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/pipe"
)

type BidirectionalConn struct {
	ctx              context.Context
	stream           BidirectionalStream
	logger           logger.ContextLogger
	cancelled        atomic.Bool
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
	readSemaphore    chan struct{}
	writeSemaphore   chan struct{}
	readDone         chan struct{}
	writeDone        chan struct{}
	doneOnce         sync.Once
	readDoneOnce     sync.Once
	writeDoneOnce    sync.Once
	onTerminate      func()
	readDeadline     pipe.Deadline
	writeDeadline    pipe.Deadline
}

func (e StreamEngine) CreateConn(ctx context.Context, l logger.ContextLogger, readWaitHeaders bool, writeWaitHeaders bool) *BidirectionalConn {
	conn := &BidirectionalConn{
		ctx:              ctx,
		logger:           l,
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
		readDone:         make(chan struct{}),
		writeDone:        make(chan struct{}),
		readDeadline:     pipe.MakeDeadline(),
		writeDeadline:    pipe.MakeDeadline(),
	}
	conn.readSemaphore <- struct{}{}
	conn.writeSemaphore <- struct{}{}
	conn.stream = e.CreateStream(&bidirectionalHandler{BidirectionalConn: conn})
	return conn
}

func (c *BidirectionalConn) waitReady(waitHeaders bool, deadline <-chan struct{}) error {
	var gate <-chan struct{}
	if waitHeaders {
		gate = c.handshake
	} else {
		gate = c.ready
	}
	select {
	case <-gate:
		return nil
	case <-c.done:
		return c.err
	case <-c.close:
		return net.ErrClosed
	case <-deadline:
		return os.ErrDeadlineExceeded
	}
}

func (c *BidirectionalConn) Start(method string, url string, headers map[string]string, priority int, endOfStream bool) error {
	c.access.Lock()
	if !c.stream.Start(method, url, headers, priority, endOfStream) {
		c.access.Unlock()
		c.terminate(os.ErrInvalid)
		return os.ErrInvalid
	}
	c.access.Unlock()
	return nil
}

func (c *BidirectionalConn) markTerminatedLocked(err error) (onTerminate func(), marked bool) {
	c.readDoneOnce.Do(func() { close(c.readDone) })
	c.writeDoneOnce.Do(func() { close(c.writeDone) })
	c.cancelled.Store(true)
	c.doneOnce.Do(func() {
		c.err = err
		close(c.done)
		onTerminate = c.onTerminate
		marked = true
	})
	return
}

func (c *BidirectionalConn) terminate(err error) {
	c.access.Lock()
	onTerminate, marked := c.markTerminatedLocked(err)
	c.access.Unlock()

	if onTerminate != nil {
		onTerminate()
	}
	if marked {
		c.stream.Destroy()
		cleanupBidirectionalStream(c.stream.ptr)
	}
}

func (c *BidirectionalConn) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	select {
	case <-c.close:
		return 0, net.ErrClosed
	case <-c.done:
		return 0, c.err
	case <-c.readSemaphore:
	}
	defer func() { c.readSemaphore <- struct{}{} }()

	if err := c.waitReady(c.readWaitHeaders, c.readDeadline.Wait()); err != nil {
		return 0, err
	}

	c.access.Lock()
	select {
	case <-c.close:
		c.access.Unlock()
		return 0, net.ErrClosed
	case <-c.done:
		c.access.Unlock()
		return 0, c.err
	default:
	}
	c.stream.Read(p)
	c.access.Unlock()

	select {
	case bytesRead := <-c.read:
		return bytesRead, nil
	case <-c.readDeadline.Wait():
		if c.cancelled.CompareAndSwap(false, true) {
			c.stream.Cancel()
		}
		for {
			select {
			case <-c.read:
			case <-c.done:
				return 0, os.ErrDeadlineExceeded
			}
		}
	case <-c.done:
		<-c.readDone
		return 0, c.err
	case <-c.close:
		<-c.readDone
		return 0, net.ErrClosed
	}
}

func (c *BidirectionalConn) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	select {
	case <-c.close:
		return 0, net.ErrClosed
	case <-c.done:
		return 0, c.err
	case <-c.writeSemaphore:
	}
	defer func() { c.writeSemaphore <- struct{}{} }()

	if err := c.waitReady(c.writeWaitHeaders, c.writeDeadline.Wait()); err != nil {
		return 0, err
	}

	c.access.Lock()
	select {
	case <-c.close:
		c.access.Unlock()
		return 0, net.ErrClosed
	case <-c.done:
		c.access.Unlock()
		return 0, c.err
	default:
	}
	c.stream.Write(p, false)
	c.access.Unlock()

	select {
	case <-c.write:
		return len(p), nil
	case <-c.writeDeadline.Wait():
		if c.cancelled.CompareAndSwap(false, true) {
			c.stream.Cancel()
		}
		for {
			select {
			case <-c.write:
			case <-c.done:
				return 0, os.ErrDeadlineExceeded
			}
		}
	case <-c.done:
		<-c.writeDone
		return 0, c.err
	case <-c.close:
		<-c.writeDone
		return 0, net.ErrClosed
	}
}

func (c *BidirectionalConn) Done() <-chan struct{} {
	return c.done
}

func (c *BidirectionalConn) setOnTerminate(fn func()) {
	c.access.Lock()
	select {
	case <-c.done:
		c.access.Unlock()
		fn()
		return
	default:
	}
	c.onTerminate = fn
	c.access.Unlock()
}

func (c *BidirectionalConn) Err() error {
	return c.err
}

func (c *BidirectionalConn) Close() error {
	c.access.Lock()

	select {
	case <-c.close:
		c.access.Unlock()
		return net.ErrClosed
	case <-c.done:
		c.access.Unlock()
		return nil
	default:
	}

	close(c.close)
	c.access.Unlock()

	if c.cancelled.CompareAndSwap(false, true) {
		c.stream.Cancel()
	}
	return nil
}

func (c *BidirectionalConn) LocalAddr() net.Addr {
	return nil
}

func (c *BidirectionalConn) RemoteAddr() net.Addr {
	return nil
}

func (c *BidirectionalConn) SetDeadline(t time.Time) error {
	c.SetReadDeadline(t)
	c.SetWriteDeadline(t)
	return nil
}

func (c *BidirectionalConn) SetReadDeadline(t time.Time) error {
	c.readDeadline.Set(t)
	return nil
}

func (c *BidirectionalConn) SetWriteDeadline(t time.Time) error {
	c.writeDeadline.Set(t)
	return nil
}

func (c *BidirectionalConn) WaitForHeaders() (map[string]string, error) {
	select {
	case <-c.handshake:
		return c.headers, nil
	case <-c.done:
		return nil, c.err
	case <-c.close:
		return nil, net.ErrClosed
	}
}

func (c *BidirectionalConn) WaitForHeadersContext(ctx context.Context) (map[string]string, error) {
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
}

func (c *bidirectionalHandler) OnStreamReady(stream BidirectionalStream) {
	c.readyOnce.Do(func() { close(c.ready) })
}

func (c *bidirectionalHandler) OnResponseHeadersReceived(stream BidirectionalStream, headers map[string]string, negotiatedProtocol string) {
	c.headers = headers
	c.logger.DebugContext(c.ctx, "response received, protocol: ", negotiatedProtocol, ", status: ", headers[":status"])
	c.handshakeOnce.Do(func() { close(c.handshake) })
}

func (c *bidirectionalHandler) OnReadCompleted(stream BidirectionalStream, bytesRead int) {
	if bytesRead == 0 {
		c.terminate(io.EOF)
		return
	}

	select {
	case <-c.close:
		c.readDoneOnce.Do(func() { close(c.readDone) })
	case <-c.done:
		c.readDoneOnce.Do(func() { close(c.readDone) })
	case c.read <- bytesRead:
	}
}

func (c *bidirectionalHandler) OnWriteCompleted(stream BidirectionalStream) {
	select {
	case <-c.close:
		c.writeDoneOnce.Do(func() { close(c.writeDone) })
	case <-c.done:
		c.writeDoneOnce.Do(func() { close(c.writeDone) })
	case c.write <- struct{}{}:
	}
}

func (c *bidirectionalHandler) OnResponseTrailersReceived(stream BidirectionalStream, trailers map[string]string) {
}

func (c *bidirectionalHandler) OnSucceeded(stream BidirectionalStream) {
	c.terminate(io.EOF)
}

func (c *bidirectionalHandler) OnFailed(stream BidirectionalStream, netError int) {
	c.logger.WarnContext(c.ctx, "stream failed: ", NetError(netError))
	c.terminate(NetError(netError))
}

func (c *bidirectionalHandler) OnCanceled(stream BidirectionalStream) {
	c.logger.DebugContext(c.ctx, "stream canceled")
	c.terminate(context.Canceled)
}
