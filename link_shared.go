//go:build !static

package cronet

// #cgo CFLAGS: -I/usr/local/include -I.
// #cgo LDFLAGS: -L/usr/local/lib -L. -lcronet
import "C"
