//go:build linux && !android && mipsle && !with_musl && !with_purego

package linux_mipsle

// #cgo LDFLAGS: -L${SRCDIR} -l:libcronet.a -latomic -ldl -lpthread -lrt -lm -lresolv
import "C"

const Version = "154.0.8037.49"
