//go:build !static && linux && (386 || arm || mipsle || mips64le)

package cronet

// #cgo LDFLAGS: -ldl -lpthread -lrt
import "C"
