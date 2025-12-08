package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
// extern void cronetRunnableRun(Cronet_RunnablePtr self);
import "C"

import (
	"sync"
	"unsafe"
)

// RunnableRunFunc is the function type for Runnable.Run callback.
type RunnableRunFunc func(self Runnable)

// NewRunnable creates a new Runnable with the given run function.
func NewRunnable(runFunc RunnableRunFunc) Runnable {
	ptr := C.Cronet_Runnable_CreateWith((*[0]byte)(C.cronetRunnableRun))
	runnableAccess.Lock()
	runnableMap[uintptr(unsafe.Pointer(ptr))] = runFunc
	runnableAccess.Unlock()
	return Runnable{ptr}
}

func (r Runnable) Destroy() {
	C.Cronet_Runnable_Destroy(r.ptr)
	runnableAccess.Lock()
	delete(runnableMap, uintptr(unsafe.Pointer(r.ptr)))
	runnableAccess.Unlock()
}

var (
	runnableAccess sync.RWMutex
	runnableMap    map[uintptr]RunnableRunFunc
)

func init() {
	runnableMap = make(map[uintptr]RunnableRunFunc)
}

//export cronetRunnableRun
func cronetRunnableRun(self C.Cronet_RunnablePtr) {
	runnableAccess.RLock()
	runFunc := runnableMap[uintptr(unsafe.Pointer(self))]
	runnableAccess.RUnlock()
	if runFunc == nil {
		panic("nil runnable")
	}
	runFunc(Runnable{self})
}
