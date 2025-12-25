//go:build ios && arm64 && !tvos && iossimulator && !with_purego

package ios_arm64_simulator

// #cgo LDFLAGS: ${SRCDIR}/libcronet.a -lresolv -framework CoreFoundation -framework CoreGraphics -framework CoreText -framework Foundation -framework Security -framework UIKit -framework CFNetwork -framework MobileCoreServices -framework SystemConfiguration -framework UniformTypeIdentifiers -framework CoreTelephony -framework CryptoTokenKit -framework LocalAuthentication
import "C"

const Version = "143.0.7499.109"
