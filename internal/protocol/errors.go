package protocol

type ErrorCode uint8

const (
	ErrLinkBreak ErrorCode = iota + 1
	ErrNoRoute
	ErrDestUnreachable
	ErrInvalidSeq
	ErrProtocolViolation
)

// String corresponds to fmt.Stringer interface
func (e ErrorCode) String() string {
	switch e {
	case ErrLinkBreak:
		return "link break"
	case ErrNoRoute:
		return "no route to destination"
	case ErrDestUnreachable:
		return "destination unreachable"
	case ErrInvalidSeq:
		return "sequence number violation"
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
