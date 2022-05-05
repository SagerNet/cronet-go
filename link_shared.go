//go:build !static

package cronet

// #cgo CFLAGS: -I.
// #cgo LDFLAGS: -L. -lcronet
import "C"
