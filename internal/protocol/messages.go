package protocol

import (
	"net/netip"
)

type MsgType uint8

const (
	HelloMsgType MsgType = iota + 1
	RREQMsgType
	RREPMsgType
	RERRMsgType
	DATAMsgType
)

// Header struct contains general fields that all other messages must have.
type Header struct{
	MsgType MsgType // Type of the message.
	SrcIP netip.Addr // IP address of sending host.
	SrcID uint64 // Snowflake id identifying sender.
	DstIP netip.Addr // IP address of intended recipient.
	DstID uint64 // Snowflake id identifying recipient.
	Timestamp uint64 // Timestamp when message was send.
	TTL uint8 // Hops remaining before discard (max 255).
	Length uint16
}

// HELLO is a message that every node send to find neighbours and signal neighbours about being alive.
type HELLO struct{
	Header

	Port uint16 // Port where nodes should send messages.
}

// RREQ is a message request that send to find route for specic node.
type RREQ struct{
	Header

	SrcSeq 	uint32 // Counter increamented with each send message, need to avoid duplicates.
	DstSeq uint32 // Counter for accepting RREP with DstSeq non less than this.
	HopCount uint8 // Count of hops to destination.
	BroadcastID uint64 // SrcSeq + DstSeq to avoid duplicates.
}

// RREP is a message reply for RREQ with data of founded node.
type RREP struct{
	Header

	Lifetime uint32 // Count of seconds route should live.
	DstSeq uint32 // Counter for choosing freshest route.
	HopCount uint8 // Count of hops to destination.
}

// RERR is a message that notifies node that something off.
type RERR struct{
	Header

	ErrCode ErrorCode // Code of the error.
}

// DATA is a message that contain user data.
type DATA struct{
	Header

	SeqNum uint32 // Sequential number of packet, need for tracking missed packages.
	Payload []byte // Message data.
}