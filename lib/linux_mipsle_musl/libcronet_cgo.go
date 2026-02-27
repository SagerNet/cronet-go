//go:build linux && !android && mipsle && with_musl && !with_purego

package linux_mipsle_musl

// #cgo LDFLAGS: -L${SRCDIR} -l:libcronet.a -latomic -ldl -lpthread -lrt -lresolv -static
import "C"

const Version = "143.0.7499.109"
