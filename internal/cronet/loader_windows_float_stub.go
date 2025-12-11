//go:build with_purego && windows && (386 || arm)

package cronet

// registerFloatFuncs is a stub for 32-bit platforms.
// purego does not support float parameters on 32-bit platforms.
func registerFloatFuncs() error {
	return nil
}
