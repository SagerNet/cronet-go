//go:build linux && !android && arm64 && !with_musl && !with_purego

package linux_arm64

// #cgo LDFLAGS: -L${SRCDIR} -l:libcronet.a -ldl -lpthread -lrt -lm -lresolv
import "C"

const Version = "143.0.7499.109"
