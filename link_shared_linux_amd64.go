//go:build !static

package cronet

// #cgo LDFLAGS: ./libcronet.so  -ldl -lpthread -lrt -Wl,-rpath,$ORIGIN
import "C"
