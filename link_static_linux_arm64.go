//go:build static

package cronet

// #cgo LDFLAGS: -ldl -lpthread -lrt -latomic -lresolv -lm
import "C"
