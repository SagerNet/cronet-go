//go:build darwin && !ios && amd64 && !with_purego

package darwin_amd64

// #cgo LDFLAGS: ${SRCDIR}/libcronet.a -lbsm -lpmenergy -lpmsample -lresolv -framework CoreFoundation -framework CoreGraphics -framework CoreText -framework Foundation -framework Security -framework ApplicationServices -framework AppKit -framework IOKit -framework OpenDirectory -framework CFNetwork -framework CoreServices -framework Network -framework SystemConfiguration -framework UniformTypeIdentifiers -framework CryptoTokenKit -framework LocalAuthentication
import "C"

const Version = "143.0.7499.109"
