//go:build linux && !android && arm && with_musl

package linux_arm_musl

// #cgo LDFLAGS: -L${SRCDIR} -l:libcronet.a -ldl -lpthread -lrt -lresolv -fuse-ld=lld -no-pie -static
import "C"

const Version = "140.0.7339.123"
