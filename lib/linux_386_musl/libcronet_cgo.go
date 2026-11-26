//go:build linux && !android && 386 && with_musl && !with_purego

package linux_386_musl

// #cgo LDFLAGS: ${SRCDIR}/libcronet.a -ldl -lpthread -lrt -lresolv -static
import "C"

const Version = "154.0.8037.49"
