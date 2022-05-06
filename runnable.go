package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"

// Runnable is an interface to run commands on the Executor.
//
// Note: In general creating Runnables should only be done by Cronet. Runnables
// created by the app don't have the ability to perform operations when the
// Runnable is being destroyed (i.e. by Cronet_Runnable_Destroy) so resource
// leaks are possible if the Runnable is posted to an Executor that is being
// shutdown with unexecuted Runnables. In controlled testing environments
// deallocation of associated resources can be performed in Run() if the
// runnable can be assumed to always be executed.
// */
type Runnable struct {
	ptr C.Cronet_RunnablePtr
}

func (r Runnable) Destroy() {
	C.Cronet_Runnable_Destroy(r.ptr)
}

func (r Runnable) Run() {
	C.Cronet_Runnable_Run(r.ptr)
}
