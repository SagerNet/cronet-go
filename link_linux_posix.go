//go:build linux && (amd64 || 386 || arm64 || arm)

package cronet

// #cgo LDFLAGS: -Wl,-mllvm,-instcombine-lower-dbg-declare=0 -flto=thin -Wl,--thinlto-jobs=all -Wl,--thinlto-cache-dir=thinlto-cache -Wl,--thinlto-cache-policy=cache_size=10%:cache_size_bytes=40g:cache_size_files=100000 -Wl,-mllvm,-import-instr-limit=5 -fwhole-program-vtables -Wl,--lto-O2
import "C"
