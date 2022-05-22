//go:build !static

package cronet

// #cgo LDFLAGS: -ldl -lpthread -lrt
import "C"
