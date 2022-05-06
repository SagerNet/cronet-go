package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"

import (
	"unsafe"
)

// QuicHint hint that |host| supports QUIC.
type QuicHint struct {
	ptr C.Cronet_QuicHintPtr
}

func NewQuicHint() QuicHint {
	return QuicHint{C.Cronet_QuicHint_Create()}
}

func (h QuicHint) Destroy() {
	C.Cronet_QuicHint_Destroy(h.ptr)
}

// SetHost set name of the host that supports QUIC.
func (h QuicHint) SetHost(host string) {
	cHost := C.CString(host)
	C.Cronet_QuicHint_host_set(h.ptr, cHost)
	C.free(unsafe.Pointer(cHost))
}

func (h QuicHint) Host() string {
	return C.GoString(C.Cronet_QuicHint_host_get(h.ptr))
}

// SetPort set port of the server that supports QUIC.
func (h QuicHint) SetPort(port int32) {
	C.Cronet_QuicHint_port_set(h.ptr, C.int32_t(port))
}

func (h QuicHint) Port() int32 {
	return int32(C.Cronet_QuicHint_port_get(h.ptr))
}

// SetAlternatePort set alternate port to use for QUIC
func (h QuicHint) SetAlternatePort(port int32) {
	C.Cronet_QuicHint_alternate_port_set(h.ptr, C.int32_t(port))
}

func (h QuicHint) AlternatePort() int32 {
	return int32(C.Cronet_QuicHint_alternate_port_get(h.ptr))
}
