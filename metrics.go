package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"

// Metrics
// Represents metrics collected for a single request. Most of these metrics are
// timestamps for events during the lifetime of the request, which can be used
// to build a detailed timeline for investigating performance.
//
// Represents metrics collected for a single request. Most of these metrics are
// timestamps for events during the lifetime of the request, which can be used
// to build a detailed timeline for investigating performance.
//
// Events happen in this order:
// <ol>
// <li>#request_start request start</li>
// <li>#dns_start DNS start</li>
// <li>#dns_end DNS end</li>
// <li>#connect_start connect start</li>
// <li>#ssl_start SSL start</li>
// <li>#ssl_end SSL end</li>
// <li>#connect_end connect end</li>
// <li>#sending_start sending start</li>
// <li>#sending_end sending end</li>
// <li>#response_start response start</li>
// <li>#request_end request end</li>
// </ol>
//
// Start times are reported as the time when a request started blocking on the
// event, not when the event actually occurred, with the exception of push
// start and end. If a metric is not meaningful or not available, including
// cases when a request finished before reaching that stage, start and end
// times will be null. If no time was spent blocking on an event, start and end
// will be the same time.
//
// Timestamps are recorded using a clock that is guaranteed not to run
// backwards. All timestamps are correct relative to the system clock at the
// time of request start, and taking the difference between two timestamps will
// give the correct difference between the events. In order to preserve this
// property, timestamps for events other than request start are not guaranteed
// to match the system clock at the times they represent.
//
// Most timing metrics are taken from
// <a
// href="https://cs.chromium.org/chromium/src/net/base/load_timing_info.h">LoadTimingInfo</a>,
// which holds the information for <a href="http://w3c.github.io/navigation-timing/"></a> and
// <a href="https://www.w3.org/TR/resource-timing/"></a>.
type Metrics struct {
	ptr C.Cronet_MetricsPtr
}

// RequestStart
// Time when the request started, which corresponds to calling
// Cronet_UrlRequest_Start(). This timestamp will match the system clock at
// the time it represents.
func (m Metrics) RequestStart() DateTime {
	return DateTime{C.Cronet_Metrics_request_start_get(m.ptr)}
}

// DNSStart
// Time when DNS lookup started. This and DNSEnd will be set to
// non-null regardless of whether the result came from a DNS server or the
// local cache. Will equal null if the socket was reused (see SocketReused).
func (m Metrics) DNSStart() DateTime {
	return DateTime{C.Cronet_Metrics_dns_start_get(m.ptr)}
}

// DNSEnd
// Time when DNS lookup finished. This and DNSStart will return
// non-null regardless of whether the result came from a DNS server or the
// local cache. Will equal null if the socket was reused (see SocketReused).
func (m Metrics) DNSEnd() DateTime {
	return DateTime{C.Cronet_Metrics_dns_end_get(m.ptr)}
}

// ConnectStart
// Time when connection establishment started, typically when DNS resolution
// finishes. Will equal null if the socket was reused (see SocketReused).
func (m Metrics) ConnectStart() DateTime {
	return DateTime{C.Cronet_Metrics_connect_start_get(m.ptr)}
}

// ConnectEnd
// Time when connection establishment finished, after TCP connection is
// established and, if using HTTPS, SSL handshake is completed. For QUIC
// 0-RTT, this represents the time of handshake confirmation and might happen
// later than SendingStart. Will equal null if the socket was
// reused (see SocketReused).
func (m Metrics) ConnectEnd() DateTime {
	return DateTime{C.Cronet_Metrics_connect_end_get(m.ptr)}
}

// SSLStart
// Time when SSL handshake started. For QUIC, this will be the same time as
// ConnectStart. Will equal null if SSL is not used or if the
// socket was reused (see SocketReused).
func (m Metrics) SSLStart() DateTime {
	return DateTime{C.Cronet_Metrics_ssl_start_get(m.ptr)}
}

// SSLEnd
// Time when SSL handshake finished. For QUIC, this will be the same time as
// ConnectEnd. Will equal null if SSL is not used or if the socket
// was reused (see SocketReused).
func (m Metrics) SSLEnd() DateTime {
	return DateTime{C.Cronet_Metrics_ssl_end_get(m.ptr)}
}

// SendingStart
// Time when sending HTTP request headers started.
//
// Will equal null if the request failed or was canceled before sending
// started.
func (m Metrics) SendingStart() DateTime {
	return DateTime{C.Cronet_Metrics_sending_start_get(m.ptr)}
}

// SendingEnd
// Time when sending HTTP request body finished. (Sending request body
// happens after sending request headers.)
//
// Will equal null if the request failed or was canceled before sending
// ended.
func (m Metrics) SendingEnd() DateTime {
	return DateTime{C.Cronet_Metrics_sending_end_get(m.ptr)}
}

// PushStart
// Time when first byte of HTTP/2 server push was received.  Will equal
// null if server push is not used.
func (m Metrics) PushStart() DateTime {
	return DateTime{C.Cronet_Metrics_push_start_get(m.ptr)}
}

// PushEnd
// Time when last byte of HTTP/2 server push was received.  Will equal
// null if server push is not used.
func (m Metrics) PushEnd() DateTime {
	return DateTime{C.Cronet_Metrics_push_end_get(m.ptr)}
}

// ResponseStart
// Time when the end of the response headers was received.
//
// Will equal null if the request failed or was canceled before the response
// started.
func (m Metrics) ResponseStart() DateTime {
	return DateTime{C.Cronet_Metrics_response_start_get(m.ptr)}
}

// ResponseEnd
// Time when the request finished.
func (m Metrics) ResponseEnd() DateTime {
	return DateTime{C.Cronet_Metrics_request_end_get(m.ptr)}
}

// SocketReused
// True if the socket was reused from a previous request, false otherwise.
// In HTTP/2 or QUIC, if streams are multiplexed in a single connection, this
// will be {@code true} for all streams after the first.  When {@code true},
// DNS, connection, and SSL times will be null.
func (m Metrics) SocketReused() bool {
	return bool(C.Cronet_Metrics_socket_reused_get(m.ptr))
}

// SentByteCount
// Returns total bytes sent over the network transport layer, or -1 if not
// collected.
func (m Metrics) SentByteCount() int64 {
	return int64(C.Cronet_Metrics_sent_byte_count_get(m.ptr))
}

// ReceivedByteCount
// Total bytes received over the network transport layer, or -1 if not
// collected. Number of bytes does not include any previous redirects.
func (m Metrics) ReceivedByteCount() int64 {
	return int64(C.Cronet_Metrics_received_byte_count_get(m.ptr))
}
