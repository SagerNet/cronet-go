package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
// extern int64_t cronetUploadDataProviderGetLength(Cronet_UploadDataProviderPtr self);
// extern void cronetUploadDataProviderRead(Cronet_UploadDataProviderPtr self, Cronet_UploadDataSinkPtr upload_data_sink, Cronet_BufferPtr buffer);
// extern void cronetUploadDataProviderRewind(Cronet_UploadDataProviderPtr self, Cronet_UploadDataSinkPtr upload_data_sink);
// extern void cronetUploadDataProviderClose(Cronet_UploadDataProviderPtr self);
import "C"

import (
	"sync"
	"unsafe"
)

func NewUploadDataProvider(handler UploadDataProviderHandler) UploadDataProvider {
	ptr := C.Cronet_UploadDataProvider_CreateWith(
		(*[0]byte)(C.cronetUploadDataProviderGetLength),
		(*[0]byte)(C.cronetUploadDataProviderRead),
		(*[0]byte)(C.cronetUploadDataProviderRewind),
		(*[0]byte)(C.cronetUploadDataProviderClose),
	)
	uploadDataAccess.Lock()
	uploadDataProviderMap[uintptr(unsafe.Pointer(ptr))] = handler
	uploadDataAccess.Unlock()
	return UploadDataProvider{ptr}
}

func (p UploadDataProvider) Destroy() {
	C.Cronet_UploadDataProvider_Destroy(p.ptr)
	uploadDataAccess.Lock()
	delete(uploadDataProviderMap, uintptr(unsafe.Pointer(p.ptr)))
	uploadDataAccess.Unlock()
}

var (
	uploadDataAccess      sync.RWMutex
	uploadDataProviderMap map[uintptr]UploadDataProviderHandler
)

func init() {
	uploadDataProviderMap = make(map[uintptr]UploadDataProviderHandler)
}

func instanceOfUploadDataProvider(self C.Cronet_UploadDataProviderPtr) UploadDataProviderHandler {
	uploadDataAccess.RLock()
	defer uploadDataAccess.RUnlock()
	provider := uploadDataProviderMap[uintptr(unsafe.Pointer(self))]
	if provider == nil {
		panic("nil data provider")
	}
	return provider
}

//export cronetUploadDataProviderGetLength
func cronetUploadDataProviderGetLength(self C.Cronet_UploadDataProviderPtr) C.int64_t {
	return C.int64_t(instanceOfUploadDataProvider(self).Length(UploadDataProvider{self}))
}

//export cronetUploadDataProviderRead
func cronetUploadDataProviderRead(self C.Cronet_UploadDataProviderPtr, sink C.Cronet_UploadDataSinkPtr, buffer C.Cronet_BufferPtr) {
	instanceOfUploadDataProvider(self).Read(UploadDataProvider{self}, UploadDataSink{sink}, Buffer{buffer})
}

//export cronetUploadDataProviderRewind
func cronetUploadDataProviderRewind(self C.Cronet_UploadDataProviderPtr, sink C.Cronet_UploadDataSinkPtr) {
	instanceOfUploadDataProvider(self).Rewind(UploadDataProvider{self}, UploadDataSink{sink})
}

//export cronetUploadDataProviderClose
func cronetUploadDataProviderClose(self C.Cronet_UploadDataProviderPtr) {
	instanceOfUploadDataProvider(self).Close(UploadDataProvider{self})
}
