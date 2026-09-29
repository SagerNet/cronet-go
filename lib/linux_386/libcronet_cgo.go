//go:build linux && !android && 386 && !with_musl && !with_purego

package linux_386

// #cgo LDFLAGS: -L${SRCDIR} -l:libcronet.a -ldl -lpthread -lrt -lm -lresolv
import "C"

const Version = "154.0.8037.49"
