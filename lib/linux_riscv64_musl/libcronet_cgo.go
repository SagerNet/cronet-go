//go:build linux && !android && riscv64 && with_musl && !with_purego

package linux_riscv64_musl

// #cgo LDFLAGS: -L${SRCDIR} -l:libcronet.a -ldl -lpthread -lrt -lresolv -static
import "C"

const Version = "154.0.8037.49"
