//go:build static

package cronet

// #cgo LDFLAGS: ./libcronet_static.a  -ldl -lpthread -lrt -latomic -lresolv -lm
import "C"
