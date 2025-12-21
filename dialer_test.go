package cronet

import (
	"sync"
	"testing"
)

func TestDialerMapCleanup(t *testing.T) {
	engine := NewEngine()

	engine.SetDialer(func(address string, port uint16) int {
		return -104 // ERR_CONNECTION_FAILED
	})

	dialerAccess.RLock()
	_, exists := dialerMap[engine.ptr]
	dialerAccess.RUnlock()

	if !exists {
		t.Error("dialer not registered in dialerMap")
	}

	engine.Destroy()

	dialerAccess.RLock()
	_, exists = dialerMap[engine.ptr]
	dialerAccess.RUnlock()

	if exists {
		t.Error("dialer not cleaned up after Engine.Destroy()")
	}
}

func TestUDPDialerMapCleanup(t *testing.T) {
	engine := NewEngine()

	engine.SetUDPDialer(func(address string, port uint16) (int, string, uint16) {
		return -104, "", 0 // ERR_CONNECTION_FAILED
	})

	udpDialerAccess.RLock()
	_, exists := udpDialerMap[engine.ptr]
	udpDialerAccess.RUnlock()

	if !exists {
		t.Error("dialer not registered in udpDialerMap")
	}

	engine.Destroy()

	udpDialerAccess.RLock()
	_, exists = udpDialerMap[engine.ptr]
	udpDialerAccess.RUnlock()

	if exists {
		t.Error("dialer not cleaned up after Engine.Destroy()")
	}
}

func TestSetDialerNil(t *testing.T) {
	engine := NewEngine()
	defer engine.Destroy()

	engine.SetDialer(func(address string, port uint16) int {
		return -104
	})

	dialerAccess.RLock()
	_, exists := dialerMap[engine.ptr]
	dialerAccess.RUnlock()

	if !exists {
		t.Error("dialer not registered")
	}

	engine.SetDialer(nil)

	dialerAccess.RLock()
	_, exists = dialerMap[engine.ptr]
	dialerAccess.RUnlock()

	if exists {
		t.Error("dialer not removed after SetDialer(nil)")
	}
}

func TestSetUDPDialerNil(t *testing.T) {
	engine := NewEngine()
	defer engine.Destroy()

	engine.SetUDPDialer(func(address string, port uint16) (int, string, uint16) {
		return -104, "", 0
	})

	udpDialerAccess.RLock()
	_, exists := udpDialerMap[engine.ptr]
	udpDialerAccess.RUnlock()

	if !exists {
		t.Error("dialer not registered")
	}

	engine.SetUDPDialer(nil)

	udpDialerAccess.RLock()
	_, exists = udpDialerMap[engine.ptr]
	udpDialerAccess.RUnlock()

	if exists {
		t.Error("dialer not removed after SetUDPDialer(nil)")
	}
}

func TestSetDialerOverwrite(t *testing.T) {
	engine := NewEngine()
	defer engine.Destroy()

	callCount1 := 0
	callCount2 := 0

	engine.SetDialer(func(address string, port uint16) int {
		callCount1++
		return -104
	})

	engine.SetDialer(func(address string, port uint16) int {
		callCount2++
		return -102
	})

	dialerAccess.RLock()
	count := 0
	for k := range dialerMap {
		if k == engine.ptr {
			count++
		}
	}
	dialerAccess.RUnlock()

	if count != 1 {
		t.Errorf("expected 1 entry in dialerMap, got %d", count)
	}
}

func TestDialerConcurrentAccess(t *testing.T) {
	engine := NewEngine()
	defer engine.Destroy()

	var wg sync.WaitGroup
	iterations := 100

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if n%2 == 0 {
				engine.SetDialer(func(address string, port uint16) int {
					return -104
				})
			} else {
				engine.SetDialer(nil)
			}
		}(i)
	}

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dialerAccess.RLock()
			_ = dialerMap[engine.ptr]
			dialerAccess.RUnlock()
		}()
	}

	wg.Wait()

	dialerAccess.RLock()
	count := 0
	for k := range dialerMap {
		if k == engine.ptr {
			count++
		}
	}
	dialerAccess.RUnlock()

	if count > 1 {
		t.Errorf("dialerMap has duplicate entries for engine: %d", count)
	}
}

func TestMultipleEnginesDialers(t *testing.T) {
	engine1 := NewEngine()
	engine2 := NewEngine()

	engine1.SetDialer(func(address string, port uint16) int {
		return -104
	})

	engine2.SetDialer(func(address string, port uint16) int {
		return -102
	})

	dialerAccess.RLock()
	_, exists1 := dialerMap[engine1.ptr]
	_, exists2 := dialerMap[engine2.ptr]
	dialerAccess.RUnlock()

	if !exists1 || !exists2 {
		t.Error("both dialers should be registered")
	}

	engine1.Destroy()

	dialerAccess.RLock()
	_, exists1 = dialerMap[engine1.ptr]
	_, exists2 = dialerMap[engine2.ptr]
	dialerAccess.RUnlock()

	if exists1 {
		t.Error("engine1's dialer should be removed")
	}
	if !exists2 {
		t.Error("engine2's dialer should still exist")
	}

	engine2.Destroy()
}
