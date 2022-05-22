//go:build mipsle || mips64le

package cronet

// #cgo LDFLAGS: -Wl,--strip-debug
import "C"
