package cronet

import (
	"sync"
	"sync/atomic"
)

var (
	socketCloseCounter   atomic.Uint64
	socketCloseCallbacks sync.Map
)

func registerSocketClose(callback func()) uint64 {
	if callback == nil {
		return 0
	}
	id := socketCloseCounter.Add(1)
	socketCloseCallbacks.Store(id, callback)
	return id
}

func notifySocketClose(id uint64) {
	callback, loaded := socketCloseCallbacks.LoadAndDelete(id)
	if loaded {
		callback.(func())()
	}
}
