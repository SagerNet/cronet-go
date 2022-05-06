package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"

import (
	"reflect"
	"unsafe"
)

// Buffer provided by the application to read and write data.
type Buffer struct {
	ptr C.Cronet_BufferPtr
}

func NewBuffer() Buffer {
	return Buffer{C.Cronet_Buffer_Create()}
}

func (b Buffer) Destroy() {
	C.Cronet_Buffer_Destroy(b.ptr)
}

// InitWithDataAndCallback initialize Buffer with raw buffer |data| of |size| allocated by the app.
// The |callback| is invoked when buffer is destroyed.
func (b Buffer) InitWithDataAndCallback(data []byte, callback BufferCallback) {
	C.Cronet_Buffer_InitWithDataAndCallback(b.ptr, C.Cronet_RawDataPtr(unsafe.Pointer(&data[0])), C.uint64_t(len(data)), callback.ptr)
}

// InitWithAlloc initialize Buffer by allocating buffer of |size|.
// The content of allocated data is not initialized.
func (b Buffer) InitWithAlloc(size int64) {
	C.Cronet_Buffer_InitWithAlloc(b.ptr, C.uint64_t(size))
}

// Size return size of data owned by this buffer.
func (b Buffer) Size() int64 {
	return int64(C.Cronet_Buffer_GetSize(b.ptr))
}

// Data return raw pointer to |data| owned by this buffer.
func (b Buffer) Data() unsafe.Pointer {
	return unsafe.Pointer(C.Cronet_Buffer_GetData(b.ptr))
}

func (b Buffer) DataSlice() []byte {
	size := int(b.Size())
	hdr := reflect.SliceHeader{
		Data: uintptr(b.Data()),
		Len:  size,
		Cap:  size,
	}
	return *(*[]byte)(unsafe.Pointer(&hdr))
}
