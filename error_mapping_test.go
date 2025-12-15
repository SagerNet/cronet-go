package cronet

import (
	"errors"
	"os"
	"syscall"
	"testing"
)

func TestMapDialErrorToNetError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{"nil error", nil, 0},
		{"connection refused syscall", &os.SyscallError{Syscall: "connect", Err: syscall.ECONNREFUSED}, -102},
		{"timeout syscall", &os.SyscallError{Syscall: "connect", Err: syscall.ETIMEDOUT}, -118},
		{"network unreachable syscall", &os.SyscallError{Syscall: "connect", Err: syscall.ENETUNREACH}, -109},
		{"host unreachable syscall", &os.SyscallError{Syscall: "connect", Err: syscall.EHOSTUNREACH}, -109},
		{"connection reset syscall", &os.SyscallError{Syscall: "read", Err: syscall.ECONNRESET}, -101},
		{"connection aborted syscall", &os.SyscallError{Syscall: "read", Err: syscall.ECONNABORTED}, -103},
		{"connection refused string", errors.New("connection refused"), -102},
		{"timeout string", errors.New("i/o timeout"), -118},
		{"connection timed out string", errors.New("connection timed out"), -118},
		{"network unreachable string", errors.New("network is unreachable"), -109},
		{"no route string", errors.New("no route to host"), -109},
		{"connection reset string", errors.New("connection reset by peer"), -101},
		{"unknown error", errors.New("some unknown error"), -104},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapDialErrorToNetError(tt.err)
			if result != tt.expected {
				t.Errorf("mapDialErrorToNetError(%v) = %d, want %d", tt.err, result, tt.expected)
			}
		})
	}
}

func TestMapDialErrorToNetError_WrappedError(t *testing.T) {
	// Test error wrapping
	baseErr := syscall.ECONNREFUSED
	wrappedErr := &os.SyscallError{Syscall: "connect", Err: baseErr}

	result := mapDialErrorToNetError(wrappedErr)
	if result != -102 {
		t.Errorf("wrapped ECONNREFUSED: got %d, want -102", result)
	}
}
