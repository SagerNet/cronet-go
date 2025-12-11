//go:build with_purego

package cronet

import "github.com/sagernet/cronet-go/internal/cronet"

// LoadLibrary loads the cronet shared library from the given path.
// If path is empty, it searches in standard locations.
// This function is optional - the library will be automatically loaded
// on first use from standard locations. Use this function only if you
// need to load from a custom path or want explicit error handling.
// This function is safe to call multiple times; subsequent calls are no-ops.
func LoadLibrary(path string) error {
	return cronet.LoadLibrary(path)
}
