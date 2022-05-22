//go:build !static

package cronet

// #cgo LDFLAGS: -ldl -lpthread
import "C"
