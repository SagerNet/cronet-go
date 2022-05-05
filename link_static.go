//go:build static

package cronet

// #cgo CFLAGS: -I/usr/local/include -I.
// #cgo LDFLAGS: -fuse-ld=lld -Wl,--as-needed -L/usr/local/lib -L. -lcronet_static -ldl -lpthread -lrt -lresolv -lm
import "C"
