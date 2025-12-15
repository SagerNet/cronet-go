//go:build with_purego

package windows_amd64

import (
	_ "embed"
	_ "unsafe"
)

//go:embed libcronet.dll
var embeddedDLL []byte

//go:linkname cronetEmbeddedDLL github.com/sagernet/cronet-go/internal/cronet.embeddedDLL
var cronetEmbeddedDLL []byte

const Version = "140.0.7339.123"

func init() {
	cronetEmbeddedDLL = embeddedDLL
}
