//go:build ios && arm64 && tvos && tvossimulator

package tvos_arm64_simulator

// #cgo LDFLAGS: ${SRCDIR}/libcronet.a -lresolv -framework CoreFoundation -framework CoreGraphics -framework CoreText -framework Foundation -framework Security -framework UIKit -framework CFNetwork -framework MobileCoreServices -framework SystemConfiguration -framework UniformTypeIdentifiers -framework CryptoTokenKit
import "C"

const Version = "140.0.7339.123"
