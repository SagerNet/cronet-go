//go:build linux && !android && mipsle && with_musl && !with_purego

package linux_mipsle_musl

// #cgo LDFLAGS: ${SRCDIR}/libcronet.a -latomic -ldl -lpthread -lrt -lresolv -static
import "C"

const Version = "154.0.8037.49"
