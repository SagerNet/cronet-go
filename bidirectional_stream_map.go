package cronet

import (
	"sync"
	"sync/atomic"
)

// bidirectionalStreamEntry holds a callback and its destroyed state.
// The destroyed flag is used to handle async destruction - callbacks
// may be invoked after Destroy() returns due to Cronet's async API.
type bidirectionalStreamEntry struct {
	callback  BidirectionalStreamCallback
	destroyed atomic.Bool
}

var (
	bidirectionalStreamAccess sync.RWMutex
	bidirectionalStreamMap    map[uintptr]*bidirectionalStreamEntry
)

func init() {
	bidirectionalStreamMap = make(map[uintptr]*bidirectionalStreamEntry)
}

// instanceOfBidirectionalStreamCallback returns the callback for the given stream pointer.
// Returns nil if the stream was destroyed or not found. Callers should silently
// return on nil as post-destroy callbacks are expected async API behavior.
func instanceOfBidirectionalStreamCallback(ptr uintptr) BidirectionalStreamCallback {
	bidirectionalStreamAccess.RLock()
	defer bidirectionalStreamAccess.RUnlock()
	entry := bidirectionalStreamMap[ptr]
	if entry == nil || entry.destroyed.Load() {
		return nil
	}
	return entry.callback
}

// cleanupBidirectionalStream removes the stream entry from the map.
// Should be called from terminal callbacks (OnSucceeded/OnFailed/OnCanceled).
func cleanupBidirectionalStream(ptr uintptr) {
	bidirectionalStreamAccess.Lock()
	delete(bidirectionalStreamMap, ptr)
	bidirectionalStreamAccess.Unlock()
}
