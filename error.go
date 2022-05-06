package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"

// Error is the base error passed to URLRequestCallbackHandler.OnFailed().
type Error struct {
	ptr C.Cronet_ErrorPtr
}

type ErrorCode int

const (
	// ErrorCodeErrorCallback indicating the error returned by app callback.
	ErrorCodeErrorCallback ErrorCode = 0

	// ErrorCodeErrorHostnameNotResolved indicating the host being sent the request could not be resolved to an IP address.
	ErrorCodeErrorHostnameNotResolved ErrorCode = 1

	// ErrorCodeErrorInternetDisconnected indicating the device was not connected to any network.
	ErrorCodeErrorInternetDisconnected ErrorCode = 2

	// ErrorCodeErrorNetworkChanged indicating that as the request was processed the network configuration changed.
	ErrorCodeErrorNetworkChanged ErrorCode = 3

	// ErrorCodeErrorTimedOut indicating a timeout expired. Timeouts expiring while attempting to connect will
	// be reported as the more specific ErrorCodeErrorConnectionTimedOut.
	ErrorCodeErrorTimedOut ErrorCode = 4

	// ErrorCodeErrorConnectionClosed indicating the connection was closed unexpectedly.
	ErrorCodeErrorConnectionClosed ErrorCode = 5

	// ErrorCodeErrorConnectionTimedOut indicating the connection attempt timed out.
	ErrorCodeErrorConnectionTimedOut ErrorCode = 6

	// ErrorCodeErrorConnectionRefused indicating the connection attempt was refused.
	ErrorCodeErrorConnectionRefused ErrorCode = 7

	// ErrorCodeErrorConnectionReset indicating the connection was unexpectedly reset.
	ErrorCodeErrorConnectionReset ErrorCode = 8

	// ErrorCodeErrorAddressUnreachable indicating the IP address being contacted is unreachable,
	// meaning there is no route to the specified host or network.
	ErrorCodeErrorAddressUnreachable ErrorCode = 9

	// ErrorCodeErrorQuicProtocolFailed indicating an error related to the <a href="https://www.chromium.org/quic">
	// <a>QUIC</a> protocol. When Error.ErrorCode() is this code, see
	// Error.QuicDetailedErrorCode() for more information.
	ErrorCodeErrorQuicProtocolFailed ErrorCode = 10

	// ErrorCodeErrorOther indicating another type of error was encountered.
	// |Error.InternalErrorCode()| can be consulted to get a more specific cause.
	ErrorCodeErrorOther ErrorCode = 11
)

// ErrorCode return the error code, one of ErrorCode values.
func (e Error) ErrorCode() ErrorCode {
	return ErrorCode(C.Cronet_Error_error_code_get(e.ptr))
}

// Message explaining the error.
func (e Error) Message() string {
	return C.GoString(C.Cronet_Error_message_get(e.ptr))
}

// InternalErrorCode is the cronet internal error code. This may provide more specific error
// diagnosis than ErrorCode(), but the constant values may change over time.
// See
// <a href=https://chromium.googlesource.com/chromium/src/+/main/net/base/net_error_list.h> here</a>
// for the latest list of values.
func (e Error) InternalErrorCode() int {
	return int(C.Cronet_Error_internal_error_code_get(e.ptr))
}

// Retryable |true| if retrying this request right away might succeed, |false|
// otherwise. For example, is |true| when ErrorCode() is ErrorCodeErrorNetworkChanged
// because trying the request might succeed using the new
// network configuration, but |false| when ErrorCode() is
// ErrorCodeErrorInternetDisconnected because retrying the request right away will
// encounter the same failure (instead retrying should be delayed until device regains
// network connectivity).
func (e Error) Retryable() bool {
	return bool(C.Cronet_Error_immediately_retryable_get(e.ptr))
}

// QuicDetailedErrorCode contains detailed <a href="https://www.chromium.org/quic">QUIC</a> error code from
// <a href="https://cs.chromium.org/search/?q=symbol:%5CbQuicErrorCode%5Cb">
// QuicErrorCode</a> when the ErrorCode() code is ErrorCodeErrorQuicProtocolFailed.
func (e Error) QuicDetailedErrorCode() int {
	return int(C.Cronet_Error_quic_detailed_error_code_get(e.ptr))
}
