//go:build dummy

// This file exists purely to prevent the Go toolchain from stripping
// away the C source directories and files when `go mod vendor` is used
// to populate a `vendor/` directory of a project depending on this package.

package cronet

import (
	// Prevent go tooling from stripping out the c source files.
	_ "github.com/sagernet/cronet-go/include"
)
