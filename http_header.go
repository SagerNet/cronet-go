package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"
import "unsafe"

// HTTPHeader is a single HTTP request or response header
type HTTPHeader struct {
	ptr C.Cronet_HttpHeaderPtr
}

func NewHTTPHeader() HTTPHeader {
	return HTTPHeader{C.Cronet_HttpHeader_Create()}
}

func (h HTTPHeader) Destroy() {
	C.Cronet_HttpHeader_Destroy(h.ptr)
}

// SetName sets header name
func (h HTTPHeader) SetName(name string) {
	cName := C.CString(name)
	C.Cronet_HttpHeader_name_set(h.ptr, cName)
	C.free(unsafe.Pointer(cName))
}

func (h HTTPHeader) Name() string {
	return C.GoString(C.Cronet_HttpHeader_name_get(h.ptr))
}

// SetValue sts header value
func (h HTTPHeader) SetValue(value string) {
	cValue := C.CString(value)
	C.Cronet_HttpHeader_value_set(h.ptr, cValue)
	C.free(unsafe.Pointer(cValue))
}

func (h HTTPHeader) Value() string {
	return C.GoString(C.Cronet_HttpHeader_value_get(h.ptr))
}
