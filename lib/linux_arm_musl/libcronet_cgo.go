//go:build linux && !android && arm && with_musl && !with_purego

package linux_arm_musl

// #cgo LDFLAGS: ${SRCDIR}/libcronet.a -ldl -lpthread -lrt -lresolv -static
import "C"

const Version = "154.0.8037.49"
