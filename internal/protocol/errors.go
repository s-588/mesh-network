package protocol

type ErrorCode uint8

const(
	ErrNoRoute ErrorCode = iota + 1
	ErrTTLExpired
	ErrNeighborDown
	ErrRouteExpired
	ErrBlackhole
	ErrInvalidDstSeq
	ErrBroadcastStorm
)