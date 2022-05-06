package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"

type URLResponseInfo struct {
	ptr C.Cronet_UrlResponseInfoPtr
}

func (i URLResponseInfo) Destroy() {
	C.Cronet_UrlResponseInfo_Destroy(i.ptr)
}

// The URL the response is for. This is the URL after following
// redirects, so it may not be the originally requested URL
func (i URLResponseInfo) URL() string {
	return C.GoString(C.Cronet_UrlResponseInfo_url_get(i.ptr))
}

// URLChainSize The URL chain. The first entry is the originally requested URL;
// the following entries are redirects followed.
func (i URLResponseInfo) URLChainSize() int {
	return int(C.Cronet_UrlResponseInfo_url_chain_size(i.ptr))
}

func (i URLResponseInfo) URLChainAt(index int) string {
	return C.GoString(C.Cronet_UrlResponseInfo_url_chain_at(i.ptr, C.uint32_t(index)))
}

// StatusCode is the HTTP status code. When a resource is retrieved from the cache,
// whether it was revalidated or not, the original status code is returned.
func (i URLResponseInfo) StatusCode() int {
	return int(C.Cronet_UrlResponseInfo_http_status_code_get(i.ptr))
}

// StatusText is the HTTP status text of the status line. For example, if the
// request received a "HTTP/1.1 200 OK" response, this method returns "OK".
func (i URLResponseInfo) StatusText() string {
	return C.GoString(C.Cronet_UrlResponseInfo_http_status_text_get(i.ptr))
}

// HeaderSize list size of response header field and value pairs.
// The headers are in the same order they are received over the wire.
func (i URLResponseInfo) HeaderSize() int {
	return int(C.Cronet_UrlResponseInfo_all_headers_list_size(i.ptr))
}

func (i URLResponseInfo) HeaderAt(index int) HTTPHeader {
	return HTTPHeader{C.Cronet_UrlResponseInfo_all_headers_list_at(i.ptr, C.uint32_t(index))}
}

// Cached true if the response came from the cache, including
// requests that were revalidated over the network before being retrieved
// from the cache, failed otherwise.
func (i URLResponseInfo) Cached() bool {
	return bool(C.Cronet_UrlResponseInfo_was_cached_get(i.ptr))
}

// NegotiatedProtocol is the protocol (for example 'quic/1+spdy/3') negotiated with the server.
// An empty string if no protocol was negotiated, the protocol is
// not known, or when using plain HTTP or HTTPS.
func (i URLResponseInfo) NegotiatedProtocol() string {
	return C.GoString(C.Cronet_UrlResponseInfo_negotiated_protocol_get(i.ptr))
}

// ProxyServer is the proxy server that was used for the request.
func (i URLResponseInfo) ProxyServer() string {
	return C.GoString(C.Cronet_UrlResponseInfo_proxy_server_get(i.ptr))
}

// ReceivedByteCount is a minimum count of bytes received from the network to process this
// request. This count may ignore certain overheads (for example IP and
// TCP/UDP framing, SSL handshake and framing, proxy handling). This count is
// taken prior to decompression (for example GZIP and Brotli) and includes
// headers and data from all redirects.
func (i URLResponseInfo) ReceivedByteCount() int64 {
	return int64(C.Cronet_UrlResponseInfo_received_byte_count_get(i.ptr))
}
