//go:build linux && !android && arm64 && with_musl

package linux_arm64_musl

// #cgo LDFLAGS: -L${SRCDIR} -l:libcronet.a -ldl -lpthread -lrt -lresolv -static
import "C"

const Version = "140.0.7339.123"
