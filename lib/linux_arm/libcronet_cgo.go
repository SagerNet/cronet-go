//go:build linux && !android && arm && !with_musl

package linux_arm

// #cgo LDFLAGS: -L${SRCDIR} -l:libcronet.a -ldl -lpthread -lrt -lm -lresolv -fuse-ld=lld -no-pie
import "C"

const Version = "140.0.7339.123"
