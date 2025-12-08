package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"

import "unsafe"

// ExecutorExecuteFunc takes ownership of |command| and runs it synchronously or asynchronously.
// Destroys the |command| after execution, or if executor is shutting down.
type ExecutorExecuteFunc func(executor Executor, command Runnable)

// Executor is an interface provided by the app to run |command| asynchronously.
type Executor struct {
	ptr C.Cronet_ExecutorPtr
}

func (e Executor) Execute(command Runnable) {
	C.Cronet_Executor_Execute(e.ptr, command.ptr)
}

func (e Executor) SetClientContext(context unsafe.Pointer) {
	C.Cronet_Executor_SetClientContext(e.ptr, C.Cronet_ClientContext(context))
}

func (e Executor) ClientContext() unsafe.Pointer {
	return unsafe.Pointer(C.Cronet_Executor_GetClientContext(e.ptr))
}
