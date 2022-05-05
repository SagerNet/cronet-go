//go:build static

package cronet

// #cgo CFLAGS: -I.
// #cgo LDFLAGS: -fuse-ld=lld -Wl,--as-needed -L. -lcronet_static -ldl -lpthread -lrt -lresolv -lm
import "C"
