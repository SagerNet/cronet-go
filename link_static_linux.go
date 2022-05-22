//go:build static

package cronet

// #cgo LDFLAGS: ./libcronet_static.a
import "C"
