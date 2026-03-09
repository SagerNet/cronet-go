//go:build linux && !android && loong64 && with_musl && !with_purego

package linux_loong64_musl

// #cgo LDFLAGS: -L${SRCDIR} -l:libcronet.a -ldl -lpthread -lrt -lresolv -static
import "C"

const Version = "143.0.7499.109"
