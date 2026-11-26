//go:build linux && !android && arm && !with_musl && !with_purego

package linux_arm

// #cgo LDFLAGS: ${SRCDIR}/libcronet.a -ldl -lpthread -lrt -lm -lresolv
import "C"

const Version = "154.0.8037.49"
