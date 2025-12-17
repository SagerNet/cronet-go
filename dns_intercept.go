package cronet

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
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

// wrapDNSResolverWithECH wraps a DNS resolver to inject ECH config into HTTPS
// record responses for the specified server name. The echConfigGetter is called
// on each HTTPS query to get the current ECH config (allowing dynamic updates).
func wrapDNSResolverWithECH(
	resolver DNSResolverFunc,
	serverName string,
	echConfigGetter func() []byte,
) DNSResolverFunc {
	return func(ctx context.Context, request *mDNS.Msg) *mDNS.Msg {
		response := resolver(ctx, request)

		// Check if this is an HTTPS query for our server
		if len(request.Question) > 0 {
			question := request.Question[0]
			if question.Qtype == mDNS.TypeHTTPS && matchesServerName(question.Name, serverName) {
				echConfig := echConfigGetter()
				if len(echConfig) > 0 {
					return injectECHConfig(request, response, echConfig)
				}
			}
		}
		return response
	}
}

// matchesServerName checks if a DNS query name matches the server name.
// The query name is in DNS wire format (FQDN with trailing dot).
// For HTTPS records, Chromium may query in the format "_<port>._https.<name>"
// when querying for a specific port.
func matchesServerName(queryName, serverName string) bool {
	// Normalize: remove trailing dot from query name if present
	queryName = strings.TrimSuffix(queryName, ".")
	serverName = strings.TrimSuffix(serverName, ".")

	// Direct match
	if strings.EqualFold(queryName, serverName) {
		return true
	}

	// Check for port-prefixed HTTPS query format: _<port>._https.<name>
	// Example: _443._https.example.org for example.org:443
	if strings.HasPrefix(queryName, "_") {
		parts := strings.SplitN(queryName, "._https.", 2)
		if len(parts) == 2 {
			return strings.EqualFold(parts[1], serverName)
		}
	}

	return false
}

// injectECHConfig injects or replaces ECH config in an HTTPS record response.
// If the response is nil or has no HTTPS records, a synthetic response is created.
func injectECHConfig(request *mDNS.Msg, response *mDNS.Msg, echConfig []byte) *mDNS.Msg {
	if response == nil {
		response = new(mDNS.Msg)
		response.SetReply(request)
	}

	// Look for existing HTTPS records and update their ECH config
	hasHTTPS := false
	for _, rr := range response.Answer {
		if https, ok := rr.(*mDNS.HTTPS); ok {
			hasHTTPS = true
			updateECHInSVCB(&https.SVCB, echConfig)
		}
	}

	// If no HTTPS records exist, synthesize one
	if !hasHTTPS && len(request.Question) > 0 {
		queryName := request.Question[0].Name

		// Extract the actual server name from port-prefixed HTTPS queries
		// Format: _<port>._https.<server_name>. -> server_name.
		targetName := queryName
		if strings.HasPrefix(queryName, "_") {
			parts := strings.SplitN(queryName, "._https.", 2)
			if len(parts) == 2 {
				targetName = parts[1]
			}
		}

		https := &mDNS.HTTPS{
			SVCB: mDNS.SVCB{
				Hdr: mDNS.RR_Header{
					Name:   queryName,
					Rrtype: mDNS.TypeHTTPS,
					Class:  mDNS.ClassINET,
					Ttl:    300,
				},
				Priority: 1,
				Target:   targetName,
			},
		}
		https.Value = append(https.Value, &mDNS.SVCBECHConfig{ECH: echConfig})
		response.Answer = append(response.Answer, https)
	}

	return response
}

// updateECHInSVCB updates or adds ECH config in an SVCB record's key-value pairs.
func updateECHInSVCB(svcb *mDNS.SVCB, echConfig []byte) {
	// Look for existing ECH key and update it
	for i, kv := range svcb.Value {
		if _, ok := kv.(*mDNS.SVCBECHConfig); ok {
			svcb.Value[i] = &mDNS.SVCBECHConfig{ECH: echConfig}
			return
		}
	}
	// No existing ECH key, add one
	svcb.Value = append(svcb.Value, &mDNS.SVCBECHConfig{ECH: echConfig})
}
