package socket

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/s-588/mesh-network/internal/config"
	"github.com/s-588/mesh-network/internal/protocol"
	"github.com/s-588/mesh-network/internal/routing"
	"github.com/s-588/mesh-network/pkg/logger"
)

// Socket
type Socket struct {
	port  uint16
	cfg   config.AppConfig
	links map[string]*interfaceState

	seenMu    sync.Mutex
	seenRREQs map[uint64]uint64 // Key: SrcID, Value: Last BroadcastID

	incomingMsgs chan msg // incoming messages

	inboxMu   sync.RWMutex
	inboxMsgs []string // all messages

	pendingMu   sync.Mutex
	pendingMsgs map[uint64][][]byte // RREQ messages waiting for incoming RREP

	seqNum atomic.Uint32
}

type interfaceState struct {
	name  string
	iface *net.Interface
	conn  *net.UDPConn
	addr  netip.Addr
}

type msg struct {
	data  []byte
	addr  *net.UDPAddr
	iface string
}

func NewSocket(cfg config.AppConfig) (*Socket, error) {
	t := &Socket{
		cfg:          cfg,
		port:         cfg.Port,
		links:        make(map[string]*interfaceState),
		incomingMsgs: make(chan msg, 256),
		seenRREQs:    make(map[uint64]uint64),
		pendingMsgs:  make(map[uint64][][]byte),
	}

	for _, ifaceName := range cfg.Interfaces {
		iface, err := net.InterfaceByName(ifaceName)
		if err != nil {
			return t, fmt.Errorf("get interface %s: %w", ifaceName, err)
		}
		t.links[ifaceName], err = t.setupInterface(ifaceName)
		if err != nil {
			return t, fmt.Errorf("interface setup %s: %w", ifaceName, err)
		}
		slog.Info(fmt.Sprintf("Interface %s(%s) bounded", iface.Name, iface.HardwareAddr.String()))
	}

	return t, nil
}

func (t *Socket) setupInterface(name string) (*interfaceState, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}

	var primaryAddr netip.Addr
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok {
			if ipv4 := ipNet.IP.To4(); ipv4 != nil {
				primaryAddr = netip.AddrFrom4([4]byte(ipv4))
				break
			}
		}
	}

	if !primaryAddr.IsValid() {
		return nil, fmt.Errorf("no IPv4 address found for %s", name)
	}

	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var opErr error
			err := c.Control(func(fd uintptr) {
				err = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
				if err != nil {
					opErr = err
					return
				}
				err = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
				if err != nil {
					opErr = err
					return
				}
				err = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, name)
				if err != nil {
					opErr = err
					return
				}
			})
			if err != nil {
				return err
			}
			return opErr
		},
	}

	lp, err := lc.ListenPacket(context.Background(), "udp4", fmt.Sprintf("0.0.0.0:%d", t.port))
	if err != nil {
		return nil, err
	}

	return &interfaceState{
		name:  name,
		iface: iface,
		conn:  lp.(*net.UDPConn),
		addr:  primaryAddr,
	}, nil
}

func (t *Socket) handleRREQ(msg *protocol.RREQ, srcAddr netip.AddrPort, iface string) {
	if msg.SrcID == t.cfg.ID {
		slog.Debug("RREQ from myself declined",
			"from", msg.SrcID,
			"to", msg.DstID,
			"seq", msg.SrcSeq,
			"bcastID", msg.BroadcastID,
			"prev_hop", srcAddr.Addr(),
			"interface", iface,
			"hops", msg.HopCount,
			"ttl", msg.TTL)
		return
	}

	t.seenMu.Lock()
	lastID, seen := t.seenRREQs[msg.SrcID]
	if seen && msg.BroadcastID <= lastID {
		t.seenMu.Unlock()
		slog.Debug("This RREQ we alredy saw",
			"from", msg.SrcID,
			"to", msg.DstID,
			"seq", msg.SrcSeq,
			"bcastID", msg.BroadcastID,
			"prev_hop", srcAddr.Addr(),
			"interface", iface,
			"hops", msg.HopCount,
			"ttl", msg.TTL,
		)
		return
	}
	t.seenRREQs[msg.SrcID] = msg.BroadcastID
	t.seenMu.Unlock()

	slog.Info("RREQ received",
		"from", msg.SrcID,
		"to", msg.DstID,
		"seq", msg.SrcSeq,
		"bcastID", msg.BroadcastID,
		"prev_hop", srcAddr.Addr(),
		"interface", iface,
		"hops", msg.HopCount,
		"ttl", msg.TTL,
	)

	neighborID, found := routing.FindNeighbourByAddr(srcAddr.Addr())
	if !found {
		slog.Warn("Received RREQ from unknown neighbor", "ip", srcAddr.Addr())
		neighborID = msg.SrcID
	}

	// Creating reverse route for sending messages back
	addr, _ := netip.ParseAddrPort(srcAddr.String())
	reverseRoute := routing.RouteEntry{
		DstID:       msg.SrcID,
		DstSeq:      msg.SrcSeq,
		HopCount:    msg.HopCount + 1,
		NextHopID:   neighborID,
		NextHopAddr: addr,
		LastUpdate:  time.Now(),
		Lifetime:    time.Now().Add(time.Duration(t.cfg.Lifetime) * time.Second),
		Interface:   iface,
	}
	routing.UpdateRoute(reverseRoute)

	if msg.DstID == t.cfg.ID {
		slog.Info("I am the destination. Sending RREP...",
			"from", msg.SrcID,
			"to", msg.DstID,
			"seq", msg.SrcSeq,
			"bcastID", msg.BroadcastID,
			"prev_hop", srcAddr.Addr(),
			"interface", iface,
			"hops", msg.HopCount,
			"ttl", msg.TTL,
		)
		t.SendRREP(msg, addr, iface)
		return
	}

	// Broadcast RREQ because we are not the reciever
	if msg.TTL > 1 {
		msg.TTL--
		msg.HopCount++

		for name, link := range t.links {
			msg.SrcIP = link.addr

			data, err := msg.MarshalBinary()
			if err != nil {
				slog.Error("Marshal RREQ failed", "err", err)
				return
			}

			bcastAddr := &net.UDPAddr{
				IP:   net.IPv4bcast,
				Port: int(t.port),
			}

			_, err = link.conn.WriteToUDP(data, bcastAddr)
			if err != nil {
				slog.Warn("RREQ broadcast failed", "interface", name, "error", err)
			}
		}

		slog.Info("RREQ forwarded",
			"from", msg.SrcID,
			"to", msg.DstID,
			"seq", msg.SrcSeq,
			"bcastID", msg.BroadcastID,
			"prev_hop", srcAddr.Addr(),
			"interface", iface,
			"new_hops", msg.HopCount,
			"new_ttl", msg.TTL,
		)
	} else {
		slog.Warn("RREQ dropped: TTL expired", "ttl", msg.TTL)
	}
}

func (t *Socket) handleRREP(msg *protocol.RREP, from netip.AddrPort, iface string) {
	neighborID, found := routing.FindNeighbourByAddr(from.Addr())
	if !found {
		slog.Warn("Received RREP from unknown neighbor", "ip", from.Addr())
		neighborID = msg.SrcID // Fallback, but this shouldn't happen
	}

	slog.Info("RREP recieved",
		"from", msg.SrcID,
		"to", msg.DstID,
		"dst_seq", msg.DstSeq,
		"prev_hop", from.Addr(),
		"hops", msg.HopCount,
		"ttl", msg.TTL,
	)

	// Create entry in route table, it's a Forward Path
	// SrcID in RREP it's an end destination route that we tried to find.
	// NextHop is a node that send RREP.
	newRoute := routing.RouteEntry{
		DstID:       msg.SrcID,
		DstSeq:      msg.DstSeq,
		HopCount:    msg.HopCount + 1,
		NextHopID:   neighborID,
		NextHopAddr: from,
		Lifetime:    time.Now().Add(time.Duration(msg.Lifetime) * time.Second),
		Interface:   iface,
	}
	routing.UpdateRoute(newRoute)

	// If we initialized RREQ, this RREP is for us
	if msg.DstID == t.cfg.ID {
		slog.Info("Route established",
			"type", logger.LogTypeRREPReceived,
			"to", msg.SrcID,
			"via", from.Addr(),
			"hops", msg.HopCount,
		)

		t.pendingMu.Lock()
		messages, ok := t.pendingMsgs[msg.SrcID]
		if ok {
			delete(t.pendingMsgs, msg.SrcID)
			t.pendingMu.Unlock()

			slog.Info("Sending buffered messages", "count", len(messages), "to", msg.SrcID)
			for _, payload := range messages {
				t.SendData(msg.SrcID, payload)
			}
		} else {
			t.pendingMu.Unlock()
		}
		return
	}

	if route, found := routing.FindRoute(msg.DstID); found {
		msg.HopCount++
		data, _ := msg.MarshalBinary()
		t.sendToAddr(data, route.NextHopAddr, route.Interface)
		routing.AddPrecursor(msg.SrcID, route.NextHopID)
		slog.Debug("Forwarding RREP", "to", msg.DstID, "via", route.NextHopAddr, "interface", route.Interface)
	} else {
		slog.Warn("No reverse route for RREP forwarding", "to", msg.DstID)
	}
}

func (t *Socket) handleDATA(msg *protocol.DATA, from netip.AddrPort) {
	if msg.DstID == t.cfg.ID {
		slog.Info("Message recieved",
			"type", logger.LogTypeDATAReceived,
			"from", msg.SrcID,
			"payload", string(msg.Payload))
		t.inboxMu.Lock()
		t.inboxMsgs = append(t.inboxMsgs, fmt.Sprintf("Node %d: %s", msg.SrcID, string(msg.Payload)))
		t.inboxMu.Unlock()
		return
	}

	slog.Debug("Trying to forward",
		"from", msg.SrcID,
		"to", msg.DstID,
		"seq_num", msg.SeqNum,
		"prev_hop", from,
	)

	neighbour, found := routing.FindNeighbourByAddr(from.Addr())
	if found {
		// neighbour relaying on us to forward the message
		// we need to add him to precursors to later send him RERR
		routing.AddPrecursor(msg.DstID, neighbour)
		slog.Debug("Precursor added successfully", "precursor", neighbour, "for_dest", msg.DstID)
	} else {
		slog.Warn("We don't know who send DATA message, neighbour not found",
			"from", msg.SrcID,
			"to", msg.DstID,
			"seq_num", msg.SeqNum,
			"prev_hop", from,
		)
	}

	route, found := routing.FindRoute(msg.DstID)
	if found {
		if msg.TTL <= 1 {
			slog.Warn("TTL expired, msg declined",
				"from", msg.SrcID,
				"to", msg.DstID,
				"seq_num", msg.SeqNum,
			)
			return
		}
		msg.TTL--

		data, _ := msg.MarshalBinary()
		t.sendToAddr(data, route.NextHopAddr, route.Interface)

		slog.Info("Forwarding message by reverse route",
			"from", msg.SrcID,
			"to", msg.DstID,
			"seq_num", msg.SeqNum,
			"via", route.NextHopID,
			"next_hop", route.NextHopAddr,
		)
	} else {
		slog.Warn("Can't forward, route not found, msg lost",
			"from", msg.SrcID,
			"to", msg.DstID,
			"seq_num", msg.SeqNum,
		)
		t.SendRERR(neighbour, msg.DstID, protocol.ErrDestUnreachable)
	}
}

func (t *Socket) SendData(dstID uint64, payload []byte) {
	route, found := routing.FindRoute(dstID)

	if !found {
		slog.Info("Route not found, saving in queue",
			"from", t.cfg.ID,
			"to", dstID,
		)
		t.pendingMu.Lock()
		t.pendingMsgs[dstID] = append(t.pendingMsgs[dstID], payload)
		t.pendingMu.Unlock()

		t.SendRREQ(dstID)
		return
	}

	t.seqNum.Add(1)
	opts := protocol.DATAOpts{
		HeaderOpts: protocol.HeaderOpts{
			MsgType:   protocol.DATAMsgType,
			Timestamp: uint64(time.Now().Unix()),
			SrcIP:     t.links[route.Interface].addr,
			DstIP:     route.NextHopAddr.Addr(),
			SrcID:     t.cfg.ID,
			DstID:     dstID,
			TTL:       t.cfg.TTL,
		},
		Payload: payload,
		SeqNum:  t.seqNum.Load(),
	}

	dataMsg, err := protocol.NewDATA(opts)
	if err != nil {
		slog.Error("creating DATA message", "error", err,
			"from", t.cfg.ID,
			"next_hop", route.NextHopAddr.Addr(),
			"to", dstID,
		)
		return
	}
	bytes, err := dataMsg.MarshalBinary()
	if err != nil {
		slog.Error("marshaling DATA message", "error", err,
			"from", t.cfg.ID,
			"next_hop", route.NextHopAddr.Addr(),
			"to", dstID,
		)
		return
	}

	t.sendToAddr(bytes, route.NextHopAddr, route.Interface)
	slog.Info("Message sent",
		"type", logger.LogTypeDATASent,
		"to", route.DstID,
		"payload", string(dataMsg.Payload))
}

// SendRREQ initialize search of target and broadcast RREQ
func (t *Socket) SendRREQ(targetID uint64) {
	currentSeq := t.seqNum.Add(1)

	slog.Info("Initialization of route search", "to", targetID, "seq_num", currentSeq)

	var srcIP netip.Addr
	for _, link := range t.links {
		srcIP = link.addr
		break
	}

	opts := protocol.RREQOpts{
		HeaderOpts: protocol.HeaderOpts{
			MsgType:   protocol.RREQMsgType,
			SrcID:     t.cfg.ID,
			DstID:     targetID,
			SrcIP:     srcIP,
			DstIP:     netip.IPv4Unspecified(),
			Timestamp: uint64(time.Now().UnixMilli()),
			TTL:       t.cfg.TTL,
		},
		SrcSeq:   currentSeq,
		DstSeq:   0,
		HopCount: 0,
	}

	rreq, err := protocol.NewRREQ(opts)
	if err != nil {
		slog.Error("Can't create RREQ", "error", err,
			"from", srcIP,
			"to", targetID,
		)
		return
	}

	for name, link := range t.links {
		rreq.SrcIP = link.addr

		data, err := rreq.MarshalBinary()
		if err != nil {
			slog.Error("Can't marshal RREQ", "error", err,
				"from", srcIP,
				"to", targetID,
			)

			return
		}

		bcastAddr := &net.UDPAddr{
			IP:   net.IPv4bcast,
			Port: int(t.port),
		}

		_, err = link.conn.WriteToUDP(data, bcastAddr)
		if err != nil {
			slog.Warn("RREQ broadcast failed", "interface", name, "error", err)
		}
	}
}

// SendRREP send RREP to destination
func (t *Socket) SendRREP(msg *protocol.RREQ, dst netip.AddrPort, iface string) {

	mySeq := t.seqNum.Add(1)

	slog.Info("Sending RREP",
		"to", msg.SrcID,
		"next_hop", dst,
	)

	// In RREP:
	// msg.SrcID it is us, the target of the search
	// msg.DstID it is node that search for us, original RREQ sender
	opts := protocol.RREPOpts{
		HeaderOpts: protocol.HeaderOpts{
			MsgType:   protocol.RREPMsgType,
			SrcID:     t.cfg.ID,
			DstID:     msg.SrcID,
			SrcIP:     msg.SrcIP,
			DstIP:     dst.Addr(),
			Timestamp: uint64(time.Now().UnixMilli()),
			TTL:       t.cfg.TTL,
		},
		Lifetime: t.cfg.Lifetime,
		DstSeq:   mySeq,
		HopCount: 0, // Start with 0, intermediary nodes will increment
	}

	rrep, err := protocol.NewRREP(opts)
	if err != nil {
		slog.Error("Can't create RREP", "error", err,
			"to", msg.SrcID,
			"next_hop", dst,
		)
		return
	}

	data, err := rrep.MarshalBinary()
	if err != nil {
		slog.Error("Can't marshal RREP", "error", err,
			"to", msg.SrcID,
			"next_hop", dst,
		)
		return
	}

	udpAddr := &net.UDPAddr{
		IP:   dst.Addr().AsSlice(),
		Port: int(dst.Port()),
	}

	_, err = t.links[iface].conn.WriteToUDP(data, udpAddr)
	if err != nil {
		slog.Error("Can't send RREP", "error", err,
			"to", msg.SrcID,
			"next_hop", dst,
		)
	}
}

func (t *Socket) sendToAddr(data []byte, addr netip.AddrPort, iface string) {
	udpAddr := &net.UDPAddr{
		IP:   addr.Addr().AsSlice(),
		Port: int(addr.Port()),
	}

	if link, ok := t.links[iface]; ok {
		link.conn.WriteToUDP(data, udpAddr)
		return
	}

	// fallback
	for _, link := range t.links {
		link.conn.WriteToUDP(data, udpAddr)
	}
}

func (t *Socket) Start(ctx context.Context) {
	for name, link := range t.links {
		go t.listenOnConn(ctx, name, link.conn)
	}
	slog.Info("Transport started")
}

func (t *Socket) listenOnConn(ctx context.Context, name string, conn *net.UDPConn) {
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				slog.Error("Can't read from connection", "interface", name, "error", err)
				continue
			}

			data := make([]byte, n)
			copy(data, buf[:n])

			t.incomingMsgs <- msg{data: data, addr: addr, iface: name}
		}
	}
}

func (t *Socket) ProcessMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-t.incomingMsgs:
			t.handleMessage(m)
		}
	}
}

func (t *Socket) handleMessage(m msg) {
	if len(m.data) < protocol.HeaderSize {
		return
	}

	var h protocol.Header
	if err := h.UnmarshalBinary(m.data[:protocol.HeaderSize]); err != nil {
		return
	}

	var senderAddr netip.AddrPort
	if m.addr != nil {
		senderAddr = m.addr.AddrPort()
	}

	switch h.MsgType {
	case protocol.HELLOMsgType:
		var hello protocol.HELLO
		if err := hello.UnmarshalBinary(m.data); err != nil {
			slog.Error("Failed to unmarshal HELLO", "error", err)
			return
		}
		t.handleHELLO(&hello, senderAddr, m.iface)

	case protocol.RREQMsgType:
		var rreq protocol.RREQ
		if err := rreq.UnmarshalBinary(m.data); err != nil {
			slog.Error("Failed to unmarshal RREQ", "error", err)
			return
		}
		t.handleRREQ(&rreq, senderAddr, m.iface)

	case protocol.RREPMsgType:
		var rrep protocol.RREP
		if err := rrep.UnmarshalBinary(m.data); err != nil {
			slog.Error("Failed to unmarshal RREP", "error", err)
			return
		}
		t.handleRREP(&rrep, senderAddr, m.iface)

	case protocol.RERRMsgType:
		var rerr protocol.RERR
		if err := rerr.UnmarshalBinary(m.data); err != nil {
			slog.Error("Failed to unmarshal RERR", "error", err)
			return
		}
		t.handleRERR(&rerr)

	case protocol.DATAMsgType:
		var data protocol.DATA
		if err := data.UnmarshalBinary(m.data); err != nil {
			slog.Error("Failed to unmarshal DATA", "error", err)
			return
		}
		t.handleDATA(&data, senderAddr)

	default:
		slog.Info("Received unknown message",
			"msg_type", h.MsgType,
			"to", h.DstID,
			"from", h.SrcIP,
			"prev_hop", m.addr.IP,
		)
	}
}

func (t *Socket) handleRERR(msg *protocol.RERR) {
	slog.Info("Received RERR",
		"type", logger.LogTypeRRERReceived,
		"from", msg.SrcID,
		"to", msg.DstID,
		"error_code", msg.ErrCode.String(),
		"problem_node", msg.UnreachableDstID,
		"ttl", msg.TTL,
		"timestamp", time.UnixMilli(int64(msg.Timestamp)),
	)

	route, found := routing.RoutesTable.Get(msg.UnreachableDstID)
	if !found {
		// We didn't found route to unreachable ID, so don't care
		slog.Debug("route to unreachable node was not found")
		return
	}

	// We only care if the node that sent the ERR is OUR next hop for that route
	if route.NextHopID == msg.SrcID {
		slog.Warn("Received RERR, breaking route",
			"unreachable_dst", msg.UnreachableDstID,
			"reported_by", msg.SrcID,
		)

		routing.RoutesTable.Delete(msg.UnreachableDstID)

		// Send to other precursors for which we was the next hop
		// to the destination of this route
		for precursorID := range route.Precursors {
			t.SendRERR(precursorID, msg.UnreachableDstID, msg.ErrCode)
		}
	}
}

func (t *Socket) handleHELLO(msg *protocol.HELLO, from netip.AddrPort, iface string) {
	routing.UpdateNeighbour(msg.SrcID, from, iface)
}

func (t *Socket) broadcastHello() error {
	now := time.Now().UnixMilli()

	for name, link := range t.links {
		opts := protocol.HELLOOpts{
			HeaderOpts: protocol.HeaderOpts{
				MsgType:   protocol.HELLOMsgType,
				SrcID:     t.cfg.ID,
				DstID:     0,
				SrcIP:     link.addr,
				DstIP:     netip.IPv4Unspecified(),
				Timestamp: uint64(now),
				TTL:       1,
			},
			Port: t.port,
		}

		hello, err := protocol.NewHELLO(opts)
		if err != nil {
			slog.Error("Failed to create HELLO", "interface", name, "error", err)
			continue
		}

		data, err := hello.MarshalBinary()
		if err != nil {
			slog.Error("Failed to marshal HELLO", "interface", name, "error", err)
			continue
		}
		bcastAddr := &net.UDPAddr{
			IP:   net.IPv4bcast,
			Port: int(t.port),
		}

		_, err = link.conn.WriteToUDP(data, bcastAddr)
		if err != nil {
			slog.Warn("HELLO broadcast failed", "interface", name, "error", err)
		}
	}

	return nil
}

func (t *Socket) StartHelloSender(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(t.cfg.HelloInterval) * time.Second)
	defer ticker.Stop()

	slog.Info("Hello sender started")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := t.broadcastHello(); err != nil {
				slog.Error("HELLO broadcast failed", "error", err)
			}
		}
	}
}

func (t *Socket) StartNeighbourCollector(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(t.cfg.HelloInterval+1) * time.Second)
	defer ticker.Stop()

	slog.Info("Neighbour collector started")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:

			deadNeighbours := make([]uint64, 0)
			neighboursT := routing.NeighboursTable.Snapshot()
			for id, neighbour := range neighboursT {
				// miss 3 HELLO responses
				deadLine := neighbour.LastSeen.Add(time.Duration(t.cfg.HelloInterval*3) * time.Second)

				if time.Now().After(deadLine) {
					deadNeighbours = append(deadNeighbours, id)
					routing.NeighboursTable.Delete(id)

					slog.Debug("deleting missing neighbour", "id", id,
						"last_seen", neighbour.LastSeen,
						"now", time.Now(),
					)
				}
			}

			routesT := routing.RoutesTable.Snapshot()
			for dstID, route := range routesT {

				if slices.Contains(deadNeighbours, route.NextHopID) {

					slog.Warn("route is broken because of the dead neighbour",
						"dead_neighbour", route.NextHopID,
						"to", route.DstID,
						"interface", route.Interface,
						"hops", route.HopCount,
						"dst_seq", route.DstSeq,
					)

					if len(route.Precursors) == 0 {
						slog.Debug("Route broken, but NO PRECURSORS to send RERR", "unreachable_dst", dstID)
					}
					for precursorID := range route.Precursors {
						t.SendRERR(precursorID, dstID, protocol.ErrLinkBreak)
					}
					routing.RoutesTable.Delete(dstID)
				}
			}
		}
	}
}

func (t *Socket) SendRERR(precursor uint64, unreachableID uint64, errCode protocol.ErrorCode) {
	neighbor, found := routing.NeighboursTable.Get(precursor)
	if !found {
		slog.Error("Cannot send RERR, route to precursor (neighbour) was not found",
			"precursor", precursor)
		return
	}

	opts := protocol.RERROpts{
		HeaderOpts: protocol.HeaderOpts{
			MsgType:   protocol.RERRMsgType,
			SrcID:     t.cfg.ID,
			DstID:     precursor,
			SrcIP:     t.links[neighbor.Interface].addr,
			DstIP:     neighbor.Addr.Addr(),
			Timestamp: uint64(time.Now().UnixMilli()),
			TTL:       1, // RERR is strictly hop-by-hop
		},
		ErrCode:          errCode,
		UnreachableDstID: unreachableID,
	}

	rerr, err := protocol.NewRERR(opts)
	if err != nil {
		slog.Error("Failed to create RERR", "error", err)
		return
	}

	data, err := rerr.MarshalBinary()
	if err != nil {
		slog.Error("Failed to marshal RERR", "error", err)
		return
	}

	t.sendToAddr(data, neighbor.Addr, neighbor.Interface)
}

func (t *Socket) GetSeqNum() uint32 {
	return t.seqNum.Load()
}

func (t *Socket) GetInterfaces() []string {
	iface := make([]string, 0, len(t.links))
	for _, v := range (t.links) {
		iface = append(iface, v.name)
	}
	return iface
}

func (t *Socket) GetMessages() []string {
	t.inboxMu.RLock()
	defer t.inboxMu.RUnlock()

	// Возвращаем копию среза, чтобы избежать состояния гонки
	result := make([]string, len(t.inboxMsgs))
	copy(result, t.inboxMsgs)
	return result
}
