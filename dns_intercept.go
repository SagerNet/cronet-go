package cronet

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"

	mDNS "github.com/miekg/dns"
	"github.com/sagernet/sing/common/bufio"
)

const chromiumDNSUDPMaxSize = 512

func serveDNSPacketConn(ctx context.Context, conn net.PacketConn, resolver DNSResolverFunc) error {
	defer conn.Close()

	// Close the connection when context is cancelled.
	// Datagram sockets are connectionless - closing the peer doesn't unblock ReadFrom.
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	buffer := make([]byte, 64*1024)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if deadlineConn, ok := conn.(interface{ SetReadDeadline(time.Time) error }); ok {
			_ = deadlineConn.SetReadDeadline(time.Now().Add(time.Second))
		}

		n, remoteAddress, err := conn.ReadFrom(buffer)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			if netError := (*net.OpError)(nil); errors.As(err, &netError) && netError.Timeout() {
				continue
			}
			continue
		}

		var request mDNS.Msg
		if err := request.Unpack(buffer[:n]); err != nil {
			continue
		}

		response := resolver(ctx, &request)
		response = normalizeDNSResponse(&request, response)

		packed, err := response.Pack()
		if err != nil {
			continue
		}
		if len(packed) > chromiumDNSUDPMaxSize {
			truncated := truncatedDNSResponse(&request, response.Rcode)
			packed, err = truncated.Pack()
			if err != nil {
				continue
			}
		}

		// Get a net.Conn for writing:
		// - Connected sockets (Unix socketpair, framedPacketConn) implement net.Conn directly
		// - Non-connected sockets (UDP) are wrapped with NewBindPacketConn
		var writeConn net.Conn
		if c, ok := conn.(net.Conn); ok {
			writeConn = c
		} else {
			writeConn = bufio.NewBindPacketConn(conn, remoteAddress)
		}
		_, _ = writeConn.Write(packed)
	}
}

func serveDNSStreamConn(ctx context.Context, conn net.Conn, resolver DNSResolverFunc) error {
	defer conn.Close()

	// Close the connection when context is cancelled.
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		var queryLength uint16
		if err := binary.Read(conn, binary.BigEndian, &queryLength); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		if queryLength == 0 {
			return nil
		}

		query := make([]byte, int(queryLength))
		_, err := io.ReadFull(conn, query)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}

		var request mDNS.Msg
		if err := request.Unpack(query); err != nil {
			continue
		}

		response := resolver(ctx, &request)
		response = normalizeDNSResponse(&request, response)

		packed, err := response.Pack()
		if err != nil {
			continue
		}

		_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
		var lengthPrefix [2]byte
		binary.BigEndian.PutUint16(lengthPrefix[:], uint16(len(packed)))
		if _, err := conn.Write(lengthPrefix[:]); err != nil {
			return err
		}
		if _, err := conn.Write(packed); err != nil {
			return err
		}
	}
}

func normalizeDNSResponse(request *mDNS.Msg, response *mDNS.Msg) *mDNS.Msg {
	if response == nil {
		fallback := new(mDNS.Msg)
		fallback.SetReply(request)
		fallback.Rcode = mDNS.RcodeServerFailure
		return fallback
	}

	response.Id = request.Id
	response.Response = true
	if len(response.Question) == 0 {
		response.Question = request.Question
	}
	return response
}

func truncatedDNSResponse(request *mDNS.Msg, rcode int) *mDNS.Msg {
	response := new(mDNS.Msg)
	response.SetReply(request)
	response.Truncated = true
	response.Rcode = rcode
	return response
}
