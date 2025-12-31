//go:build ios && arm64 && tvos && !tvossimulator && !with_purego

package tvos_arm64

// #cgo LDFLAGS: ${SRCDIR}/libcronet.a -lresolv -framework CoreFoundation -framework CoreGraphics -framework CoreText -framework Foundation -framework Security -framework UIKit -framework CFNetwork -framework MobileCoreServices -framework SystemConfiguration -framework UniformTypeIdentifiers -framework CryptoTokenKit
import "C"

const Version = "143.0.7499.109"
