//go:build static && linux && (amd64 || 386 || arm64 || arm || mipsle || mips64le)

package cronet

// #cgo LDFLAGS: -ldl -lpthread -lrt -latomic -lresolv -lm
import "C"
