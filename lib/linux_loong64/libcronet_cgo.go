//go:build linux && !android && loong64 && !with_musl && !with_purego

package linux_loong64

// #cgo LDFLAGS: ${SRCDIR}/libcronet.a -ldl -lpthread -lrt -lm -lresolv
import "C"

const Version = "154.0.8037.49"
