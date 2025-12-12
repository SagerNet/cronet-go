//go:build with_purego

package windows_amd64

import (
	_ "embed"
)

//go:embed libcronet.dll
var EmbeddedDLL []byte

const EmbeddedVersion = "140.0.7339.123"
