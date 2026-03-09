//go:build linux && !android && mips64le && !with_musl && !with_purego

package linux_mips64le

// #cgo LDFLAGS: -L${SRCDIR} -l:libcronet.a -ldl -lpthread -lrt -lm -lresolv
import "C"

const Version = "143.0.7499.109"
