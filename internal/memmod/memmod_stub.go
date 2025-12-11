//go:build !windows

package memmod

import "errors"

// Module is a stub for non-Windows platforms.
type Module struct{}

// LoadLibrary is not supported on non-Windows platforms.
func LoadLibrary(data []byte) (*Module, error) {
	return nil, errors.New("memmod: LoadLibrary is only supported on Windows")
}

// Free is a no-op on non-Windows platforms.
func (module *Module) Free() {}

// ProcAddressByName is not supported on non-Windows platforms.
func (module *Module) ProcAddressByName(name string) (uintptr, error) {
	return 0, errors.New("memmod: ProcAddressByName is only supported on Windows")
}

// ProcAddressByOrdinal is not supported on non-Windows platforms.
func (module *Module) ProcAddressByOrdinal(ordinal uint16) (uintptr, error) {
	return 0, errors.New("memmod: ProcAddressByOrdinal is only supported on Windows")
}

// BaseAddr returns 0 on non-Windows platforms.
func (module *Module) BaseAddr() uintptr {
	return 0
}
