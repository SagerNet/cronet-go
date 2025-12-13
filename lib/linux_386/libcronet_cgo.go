//go:build linux && !android && 386 && !with_musl

package linux_386

// #cgo LDFLAGS: -L${SRCDIR} -l:libcronet.a -ldl -lpthread -lrt -lm -lresolv
import "C"

const Version = "140.0.7339.123"
