package cronet

import "strconv"

type Errno int

const (
/*ErrnoCallback             Errno = 0
  ErrnoHostnameNotResolved  Errno = 1
  ErrnoInternetDisconnected Errno = 2
  ErrnoNetworkChanged       Errno = 3
  ErrnoTimedOut             Errno = 4
  ErrnoConnectionClosed     Errno = 5
  ErrnoConnectionTimedOut   Errno = 6
  ErrnoConnectionRefused    Errno = 7
  ErrnoConnectionReset      Errno = 8
  ErrnoAddressUnreachable   Errno = 9
  ErrnoQuicProtocolFailed   Errno = 10
  ErrnoOther                Errno = 11*/
)

func (e Errno) Error() string {
	/*switch e {
	case ErrnoCallback:
		return "callback"
	case ErrnoHostnameNotResolved:
		return "hostname not resolved"
	case ErrnoInternetDisconnected:
		return "internet disconnected"
	case ErrnoNetworkChanged:
		return "network changed"
	case ErrnoTimedOut:
		return "timeout"
	case ErrnoConnectionClosed:
		return "connection closed"
	case ErrnoConnectionTimedOut:
		return "connection timeout"
	case ErrnoConnectionRefused:
		return "connection refused"
	case ErrnoConnectionReset:
		return "connection reset"
	case ErrnoAddressUnreachable:
		return "address unreachable"
	case ErrnoQuicProtocolFailed:
		return "quic protocol failed"
	case ErrnoOther:
		return "other error"
	}*/
	return "error (" + strconv.Itoa(int(e)) + ")"
}
