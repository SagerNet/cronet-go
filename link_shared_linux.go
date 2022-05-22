//go:build !static

package cronet

// #cgo LDFLAGS: ./libcronet.so -Wl,-rpath,$ORIGIN
import "C"
