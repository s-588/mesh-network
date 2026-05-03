package protocol

import (
	"net/netip"
)

type MsgType uint8

func (m MsgType) IsValid() bool {
	switch m {
	case HELLOMsgType, RREQMsgType, RREPMsgType, RERRMsgType, DATAMsgType:
		return true
	default:
		return false
	}
}

const (
	HELLOMsgType MsgType = iota + 1
	RREQMsgType
	RREPMsgType
	RERRMsgType
	DATAMsgType
)

const (
	// HeaderSize is how many bytes take all Header fields
	HeaderSize = 2 + // Length	 uint16
		1 + // MsgType   uint8
		8 + // Timestamp uint64
		1 + // TTL       uint8
		8 + // SrcID     uint64
		8 + // DstID     uint64
		4 + // SrcIPv4 netip.Addr
		4 // DstIPv4 netip.Addr

	// HELLOSize is how many bytes take all Header and HELLO fields
	HELLOSize = HeaderSize +
		2 // Port uint16

	// RREQSize is how many bytes take all Header and RREQSize fields
	RREQSize = HeaderSize +
		4 + // SrcSeq uint32
		4 + // DstSeq uint32
		1 + // HopCount uint8
		8 // BroadcastID uint64

	// RREPSize is how many bytes take all Header and RREPSize fields
	RREPSize = HeaderSize +
		4 + // Lifetime uint32
		4 + // DstSeq uint32
		1 // HopCount uint8

	RERRSize = HeaderSize +
		1 + // ErrorCode uint8
		8 // UnreachableDstID uint64
)

// Header struct contains general fields that all other messages must have.
// If the Header struct need to be changed HeaderSize constanta must be updated.
type Header struct {
	MsgType   MsgType    // Type of the message.
	Timestamp uint64     // Timestamp when message was send.
	TTL       uint8      // Hops remaining before discard (max 255). Default is 32.
	SrcID     uint64     // Snowflake id identifying sender.
	DstID     uint64     // Snowflake id identifying recipient.
	SrcIP     netip.Addr // IPv4 address of sending host.
	DstIP     netip.Addr // IPv4 address of intended recipient.
}

type HeaderOpts struct {
	MsgType      MsgType
	SrcIP, DstIP netip.Addr
	SrcID, DstID uint64
	Timestamp    uint64
	TTL          uint8
}

// NewHeader creates pointer to Header struct.
// Return error if either Src or Dst IPs are incorrect or not IPv4.
func NewHeader(opts HeaderOpts) (*Header, error) {
	if !opts.SrcIP.IsValid() || !opts.SrcIP.Is4() {
		return nil, fmt.Errorf("source IP must be IPv4: %s", opts.SrcIP.String())
	}
	if !opts.DstIP.IsValid() || !opts.DstIP.Is4() {
		return nil, fmt.Errorf("destination IP must be IPv4: %s", opts.SrcIP.String())
	}
	if !opts.MsgType.IsValid() {

	}
	h := &Header{
		MsgType:   opts.MsgType,
		SrcIP:     opts.SrcIP,
		DstIP:     opts.DstIP,
		SrcID:     opts.SrcID,
		DstID:     opts.DstID,
		Timestamp: opts.Timestamp,
		TTL:       opts.TTL,
	}
	return h, nil
}

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