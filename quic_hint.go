package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"
import "unsafe"

type QuicHint struct {
	ptr C.Cronet_QuicHintPtr
}

func NewQuicHint() *QuicHint {
	return &QuicHint{C.Cronet_QuicHint_Create()}
}

func (h *QuicHint) Destroy() {
	C.Cronet_QuicHint_Destroy(h.ptr)
}

func (h *QuicHint) GetHost() string {
	return C.GoString(C.Cronet_QuicHint_host_get(h.ptr))
}

func (h *QuicHint) SetHost(host string) {
	cHost := C.CString(host)
	C.Cronet_QuicHint_host_set(h.ptr, cHost)
	C.free(unsafe.Pointer(cHost))
}

func (h *QuicHint) GetPort() int32 {
	return int32(C.Cronet_QuicHint_port_get(h.ptr))
}

func (h *QuicHint) SetPort(port int32) {
	C.Cronet_QuicHint_port_set(h.ptr, C.int32_t(port))
}

func (h *QuicHint) GetAlternatePort() int32 {
	return int32(C.Cronet_QuicHint_alternate_port_get(h.ptr))
}

func (h *QuicHint) SetAlternatePort(port int32) {
	C.Cronet_QuicHint_alternate_port_set(h.ptr, C.int32_t(port))
}
