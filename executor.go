package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"

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
