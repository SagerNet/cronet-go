package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"

// BufferCallbackFunc invoked when |buffer| is destroyed so its app-allocated |data| can
// be freed. If a URLRequest has ownership of a Buffer and the UrlRequest is destroyed
// (e.g. URLRequest.Destroy() is called), then Cronet will call BufferCallbackFunc().
type BufferCallbackFunc func(callback BufferCallback, buffer Buffer)

// BufferCallback is app-provided callback passed to Buffer.InitWithDataAndCallback that gets invoked
// when Buffer is destroyed.
type BufferCallback struct {
	ptr C.Cronet_BufferCallbackPtr
}

func (c BufferCallback) Destroy() {
	C.Cronet_BufferCallback_Destroy(c.ptr)
}
