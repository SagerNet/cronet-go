//go:build linux && !android && arm64 && !with_musl

package linux_arm64

// #cgo LDFLAGS: -L${SRCDIR} -l:libcronet.a -ldl -lpthread -lrt -lm -lresolv -fuse-ld=lld
import "C"

const Version = "140.0.7339.123"
