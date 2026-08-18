// errors.go contains public errors of protocol
package protocol

type ErrorCode uint8

const (
	ErrLinkBreak ErrorCode = iota + 1 // When connection to node is broken
	ErrDestUnreachable // TTL expired and route to destination is not found
	ErrProtocolViolation // Incorrectly created message
)

// String corresponds to fmt.Stringer interface
func (e ErrorCode) String() string {
	switch e {
	case ErrLinkBreak:
		return "link break"
	case ErrDestUnreachable:
		return "destination unreachable"
	case ErrProtocolViolation:
		return "protocol violation"
	default:
		return "unknown error"
	}
}

// Error corresponds to default error interface
func (e ErrorCode) Error() string {
	return e.String()
}
