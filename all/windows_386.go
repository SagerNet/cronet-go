//go:build windows && 386 && with_purego

package all

import (
	_ "github.com/sagernet/cronet-go"
	_ "github.com/sagernet/cronet-go/lib/windows_386"
)
