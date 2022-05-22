//go:build !static

package cronet

// #cgo LDFLAGS: ./libcronet.so  -ldl -lm -Wl,-rpath,$ORIGIN
import "C"
