//go:build linux && !android && arm && !with_musl && !with_purego

package linux_arm

// #cgo LDFLAGS: -L${SRCDIR} -l:libcronet.a -ldl -lpthread -lrt -lm -lresolv
import "C"

const Version = "143.0.7499.109"
