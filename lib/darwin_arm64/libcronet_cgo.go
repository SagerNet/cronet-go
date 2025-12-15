//go:build darwin && !ios && arm64

package darwin_arm64

// #cgo LDFLAGS: ${SRCDIR}/libcronet.a -lbsm -lpmenergy -lpmsample -lresolv -framework CoreFoundation -framework CoreGraphics -framework CoreText -framework Foundation -framework Security -framework ApplicationServices -framework AppKit -framework IOKit -framework OpenDirectory -framework CFNetwork -framework CoreServices -framework SystemConfiguration -framework UniformTypeIdentifiers -framework CryptoTokenKit -framework LocalAuthentication
import "C"

const Version = "140.0.7339.123"
