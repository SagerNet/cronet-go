//go:build !static

package cronet

// #cgo LDFLAGS: -ldl -lm
import "C"
