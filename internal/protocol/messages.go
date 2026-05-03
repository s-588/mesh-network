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

func (h *Header) MarshalBinary() ([]byte, error) {
	if !h.SrcIP.IsValid() || !h.DstIP.IsValid() {
		return nil, fmt.Errorf("invalid IP address")
	}
	if !h.SrcIP.Is4() || !h.DstIP.Is4() {
		return nil, fmt.Errorf("only IPv4 addresses are supported")
	}

	result := make([]byte, HeaderSize)
	offset := 0

	result[offset] = byte(h.MsgType)
	offset++

	binary.BigEndian.PutUint64(result[offset:], h.Timestamp)
	offset += 8

	result[offset] = h.TTL
	offset++

	binary.BigEndian.PutUint64(result[offset:], h.SrcID)
	offset += 8

	binary.BigEndian.PutUint64(result[offset:], h.DstID)
	offset += 8

	srcIP := h.SrcIP.As4()
	copy(result[offset:], srcIP[:])
	offset += 4

	dstIP := h.DstIP.As4()
	copy(result[offset:], dstIP[:])
	offset += 4

	return result, nil
}

func (h *Header) UnmarshalBinary(data []byte) error {
	if len(data) != HeaderSize {
		return fmt.Errorf("invalid header size, got %d, want %d", len(data), HeaderSize)
	}

	offset := 0

	h.MsgType = MsgType(data[offset])
	offset++

	h.Timestamp = binary.BigEndian.Uint64(data[offset : offset+8])
	offset += 8

	h.TTL = data[offset]
	offset++

	h.SrcID = binary.BigEndian.Uint64(data[offset : offset+8])
	offset += 8

	h.DstID = binary.BigEndian.Uint64(data[offset : offset+8])
	offset += 8

	var src [4]byte
	copy(src[:], data[offset:offset+4])
	h.SrcIP = netip.AddrFrom4(src)
	offset += 4

	var dst [4]byte
	copy(dst[:], data[offset:offset+4])
	h.DstIP = netip.AddrFrom4(dst)

	return nil
}

// HELLO is a message that every node send to find neighbours and signal neighbours about being alive.
type HELLO struct {
	Header

	Port uint16 // Port where nodes should send messages.
}

type HELLOOpts struct {
    HeaderOpts            
    Port          uint16  
}

// NewHELLO creates a pointer to a HELLO struct.
// Returns an error if Header fields are invalid or Port is zero.
func NewHELLO(opts HELLOOpts) (*HELLO, error) {
    header, err := NewHeader(opts.HeaderOpts)
    if err != nil {
        return nil, err
    }

    if opts.Port == 0 {
        return nil, fmt.Errorf("HELLO Port must be non‑zero")
    }

    hello := &HELLO{
        Header: Header{
            MsgType:   header.MsgType,
            Timestamp: header.Timestamp,
            TTL:       header.TTL,
            SrcID:     header.SrcID,
            DstID:     header.DstID,
            SrcIP:     header.SrcIP,
            DstIP:     header.DstIP,
        },
        Port: opts.Port,
    }

    return hello, nil
}

// MarshalBinary encodes HELLO into a byte slice.
func (m *HELLO) MarshalBinary() ([]byte, error) {
	headerBytes, err := m.Header.MarshalBinary()
	if err != nil {
		return nil, err
	}

	// Ensure result length matches HELLOSize.
	result := make([]byte, HELLOSize)
	copy(result, headerBytes)

	offset := len(headerBytes)
	binary.BigEndian.PutUint16(result[offset:], m.Port)

	return result, nil
}

// UnmarshalBinary decodes given data into HELLO.
func (m *HELLO) UnmarshalBinary(data []byte) error {
	if len(data) != HELLOSize {
		return fmt.Errorf("invalid HELLO size, got %d, want %d", len(data), HELLOSize)
	}

	// Reuse Header.UnmarshalBinary.
	err := m.Header.UnmarshalBinary(data[:HeaderSize])
	if err != nil {
		return err
	}

	m.Port = binary.BigEndian.Uint16(data[HeaderSize:HeaderSize+2])
	return nil
}

// RREQ is a message request that send to find route for specic node.
type RREQ struct {
	Header

	SrcSeq      uint32 // Counter increamented with each send message, need to avoid duplicates.
	DstSeq      uint32 // Counter for accepting RREP with DstSeq non less than this.
	HopCount    uint8  // Count of hops to destination.
	BroadcastID uint64 // SrcSeq + DstSeq to avoid duplicates.
}

type RREQOpts struct {
	HeaderOpts

	SrcSeq   uint32
	DstSeq   uint32
	HopCount uint8
}

// NewRREQ creates a pointer to an RREQ struct.
// Returns an error if Header fields are invalid.
func NewRREQ(opts RREQOpts) (*RREQ, error) {
	header, err := NewHeader(opts.HeaderOpts)
	if err != nil {
		return nil, err
	}

	rreq := &RREQ{
		Header:      *header,
		SrcSeq:      opts.SrcSeq,
		DstSeq:      opts.DstSeq,
		HopCount:    opts.HopCount,
		BroadcastID: uint64(opts.SrcSeq + opts.DstSeq),
	}

	return rreq, nil
}

func (m *RREQ) MarshalBinary() ([]byte, error) {
	headerBytes, err := m.Header.MarshalBinary()
	if err != nil {
		return nil, err
	}

	result := make([]byte, RREQSize) // will be RREQSize
	copy(result, headerBytes)

	offset := len(headerBytes)

	binary.BigEndian.PutUint32(result[offset:], m.SrcSeq)
	offset += 4

	binary.BigEndian.PutUint32(result[offset:], m.DstSeq)
	offset += 4

	result[offset] = m.HopCount
	offset++

	binary.BigEndian.PutUint64(result[offset:], m.BroadcastID)

	return result, nil
}

func (m *RREQ) UnmarshalBinary(data []byte) error {
	if len(data) != RREQSize {
		return fmt.Errorf("invalid RREQ size, got %d, want %d", len(data), RREQSize)
	}

	err := m.Header.UnmarshalBinary(data[:HeaderSize])
	if err != nil {
		return err
	}

	offset := HeaderSize

	m.SrcSeq = binary.BigEndian.Uint32(data[offset:])
	offset += 4

	m.DstSeq = binary.BigEndian.Uint32(data[offset:])
	offset += 4

	m.HopCount = data[offset]
	offset++

	m.BroadcastID = binary.BigEndian.Uint64(data[offset:])

	return nil
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