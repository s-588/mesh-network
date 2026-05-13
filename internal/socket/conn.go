package socket

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/s-588/mesh-network/internal/config"
	"github.com/s-588/mesh-network/internal/protocol"
	"github.com/s-588/mesh-network/internal/routing"
)

// Socket
type Socket struct {
	port  uint16
	cfg   config.AppConfig
	links map[string]*interfaceState
	msgs  chan msg // incoming messages

	seenRREQs map[uint64]uint64 // Key: SrcID, Value: Last BroadcastID
	seenMu    sync.Mutex

	pendingMsgs map[uint64][][]byte // RREQ messages waiting for incoming RREP
	pendingMu   sync.Mutex

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
		cfg:         cfg,
		port:        cfg.Port,
		links:       make(map[string]*interfaceState),
		msgs:        make(chan msg, 256),
		seenRREQs:   make(map[uint64]uint64),
		pendingMsgs: make(map[uint64][][]byte),
	}

	for _, ifaceName := range cfg.Ifaces {
		iface, err := net.InterfaceByName(ifaceName)
		if err != nil {
			return t, fmt.Errorf("get interface %s: %w", ifaceName, err)
		}
		t.links[ifaceName], err = t.setupInterface(ifaceName)
		if err != nil {
			return t, fmt.Errorf("interface setup %s: %w", ifaceName, err)
		}
		slog.Info("Interface bound", "name", iface.Name, "mac", iface.HardwareAddr)
	}

	return t, nil
}

func (t *Socket) setupInterface(name string) (*interfaceState, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}

	// 1. Parse IP upfront
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

	// 2. Bind UDP Connection
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var opErr error
			err := c.Control(func(fd uintptr) {
				_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
				opErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, name)
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

func (t *Socket) handleRREQ(rreq *protocol.RREQ, srcAddr *net.UDPAddr, iface string) {
	header := rreq.Header

	// RREQ from myself
	if header.SrcID == t.cfg.ID {
		slog.Debug("RREQ from myself declined")
		return
	}

	// Ignore RREQs with broadcastID we already saw
	t.seenMu.Lock()
	lastID, seen := t.seenRREQs[header.SrcID]
	if seen && rreq.BroadcastID <= lastID {
		t.seenMu.Unlock()
		slog.Debug("This RREQ we alredy saw")
		return
	}
	t.seenRREQs[header.SrcID] = rreq.BroadcastID
	t.seenMu.Unlock()

	slog.Info("RREQ received",
		"from", header.SrcID,
		"target", header.DstID,
		"bcastID", rreq.BroadcastID,
		"hops", rreq.HopCount,
		"ttl", header.TTL)

	// Creating reverse rounte for sending messages back
	addr, _ := netip.ParseAddrPort(srcAddr.String())
	reverseRoute := routing.RouteEntry{
		DstID:       header.SrcID,
		DstSeq:      rreq.SrcSeq,
		HopCount:    rreq.HopCount + 1,
		NextHopID:   header.SrcID, // sending back to someone who sent or forwarded message
		NextHopAddr: addr,
		LastUpdate:  time.Now(),
		Lifetime:    time.Now().Add(time.Second * time.Duration(t.cfg.Lifetime)),
		Interface:   iface,
	}
	routing.UpdateRoute(reverseRoute)

	if header.DstID == t.cfg.ID {
		slog.Info("I am the destination. Sending RREP...")
		t.SendRREP(rreq, addr)
		return
	}

	// Broadcast RREQ because we are not the reciever
	if header.TTL > 1 {
		rreq.Header.TTL--
		rreq.HopCount++

		data, err := rreq.MarshalBinary()
		if err != nil {
			slog.Error("Marshal RREQ failed", "err", err)
			return
		}

		t.Broadcast(data)

		slog.Info("↪️ RREQ FORWARDED",
			"from", header.SrcID,
			"target", header.DstID,
			"new_hops", rreq.HopCount,
			"new_ttl", rreq.Header.TTL)
	} else {
		slog.Warn("RREQ dropped: TTL expired", "ttl", header.TTL)
	}
}

func (t *Socket) handleRREP(msg *protocol.RREP, from netip.AddrPort) {
	slog.Info("[RREP] Получен ответ", "dst", msg.Header.SrcID, "hops", msg.HopCount)

	// Create entry in route table, it's a Forward Path
	// SrcID in RREP it's an end destination route that we tried to find.
	// NextHop is a node that send RREP.
	newRoute := routing.RouteEntry{
		DstID:       msg.Header.SrcID,
		DstSeq:      msg.DstSeq,
		HopCount:    msg.HopCount + 1,
		NextHopID:   msg.Header.SrcID, // RREP sender
		NextHopAddr: from,
		Lifetime:    time.Now().Add(time.Duration(msg.Lifetime) * time.Millisecond),
	}
	routing.UpdateRoute(newRoute)

	// If we initialized RREQ, this RREP is for us
	if msg.DstID == t.cfg.ID {
		slog.Info("Route established", "to", msg.SrcID, "hops", msg.HopCount)

		t.pendingMu.Lock()
		messages, ok := t.pendingMsgs[msg.Header.SrcID]
		if ok {
			delete(t.pendingMsgs, msg.Header.SrcID)
			t.pendingMu.Unlock()

			slog.Info("[SEND] Sending remaining data", "count", len(messages))
			for _, payload := range messages {
				t.SendData(msg.Header.SrcID, payload)
			}
		} else {
			t.pendingMu.Unlock()
		}
		return
	}

	if route, found := routing.FindRoute(msg.Header.DstID); found {
		msg.HopCount++
		data, _ := msg.MarshalBinary()
		t.sendToAddr(data, route.NextHopAddr, route.Interface)
		slog.Debug("[FORWARD] Forwarding RREP", "to", msg.Header.DstID)
	} else {
		slog.Warn("No reverse route for RREP forwarding", "to", msg.Header.DstID)
	}
}

func (t *Socket) handleDATA(msg *protocol.DATA) {
	if msg.Header.DstID == t.cfg.ID {
		slog.Info("[RECEIVE] Message recieved",
			"from", msg.Header.SrcID,
			"payload", string(msg.Payload))
		return
	}

	slog.Debug("[DATA] Trying to forward", "target", msg.Header.DstID)

	route, found := routing.FindRoute(msg.Header.DstID)
	if found {
		if msg.Header.TTL <= 1 {
			slog.Warn("[DATA] TTL expired, msg declined")
			return
		}
		msg.Header.TTL--

		data, _ := msg.MarshalBinary()
		t.sendToAddr(data, route.NextHopAddr, route.Interface)

		slog.Info("[FORWARD]",
			"node", t.cfg.ID,
			"target", msg.Header.DstID,
			"via", route.NextHopID)
	} else {
		slog.Warn("[FORWARD] Route not found, msg lost", "dst", msg.Header.DstID)
		// TODO: RERR
	}
}

func (t *Socket) SendData(dstID uint64, payload []byte) {
	route, found := routing.FindRoute(dstID)
	fmt.Println(route)

	if !found {
		slog.Info("[BUFFER] Route not found, saving in queue", "dst", dstID)
		t.pendingMu.Lock()
		t.pendingMsgs[dstID] = append(t.pendingMsgs[dstID], payload)
		t.pendingMu.Unlock()

		t.SendRREQ(dstID)
		return
	}

	var srcIP netip.Addr
	for _, link := range t.links {
		srcIP = link.addr
		break
	}

	t.seqNum.Add(1)
	opts := protocol.DATAOpts{
		HeaderOpts: protocol.HeaderOpts{
			MsgType:   protocol.DATAMsgType,
			Timestamp: uint64(time.Now().Unix()),
			SrcIP:     srcIP,
			DstIP:     route.NextHopAddr.Addr(),
			SrcID:     t.cfg.ID,
			DstID:     dstID,
			TTL:       10,
		},
		Payload: payload,
		SeqNum:  t.seqNum.Load(),
	}

	dataMsg, err := protocol.NewDATA(opts)
	if err != nil {
		slog.Error("creating DATA message", "error", err)
		return
	}
	bytes, err := dataMsg.MarshalBinary()
	if err != nil {
		slog.Error("marshaling DATA message", "error", err)
		return
	}

	t.sendToAddr(bytes, route.NextHopAddr, route.Interface)
}

// SendRREQ initialize search of target and broadcast RREQ
func (t *Socket) SendRREQ(targetID uint64) {
	currentSeq := t.seqNum.Add(1)

	slog.Info("[RREQ] Initialization of route search", "target", targetID, "my_seq", currentSeq)

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
		slog.Error("Can't create RREQ", "error", err)
		return
	}

	data, err := rreq.MarshalBinary()
	if err != nil {
		slog.Error("Can't marshal RREQ", "error", err)
		return
	}

	t.Broadcast(data)
}

// SendRREP send RREP to destination
func (t *Socket) SendRREP(rreq *protocol.RREQ, dst netip.AddrPort) {

	mySeq := t.seqNum.Add(1)

	slog.Info("[RREP] Sending response", "to_node", rreq.Header.SrcID, "addr", dst)

	// TODO: probably need to change to interface from where RREQ come
	var srcIP netip.Addr
	for _, link := range t.links {
		srcIP = link.addr
		break
	}

	// In RREP:
	// Header.SrcID it is us, the target of the search
	// Header.DstID it is node that search for us, original RREQ sender
	opts := protocol.RREPOpts{
		HeaderOpts: protocol.HeaderOpts{
			MsgType:   protocol.RREPMsgType,
			SrcID:     t.cfg.ID,
			DstID:     rreq.Header.SrcID,
			SrcIP:     srcIP,
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
		slog.Error("Can't create RREP", "error", err)
		return
	}

	data, err := rrep.MarshalBinary()
	if err != nil {
		slog.Error("Can't marshal RREP", "error", err)
		return
	}

	udpAddr := &net.UDPAddr{
		IP:   dst.Addr().AsSlice(),
		Port: int(dst.Port()),
	}

	// TODO: change to interface where RREQ comes from
	for _, link := range t.links {
		_, err := link.conn.WriteToUDP(data, udpAddr)
		if err != nil {
			slog.Error("Can't send RREP", "error", err)
		}
		break
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
	slog.Info("Transport started", "interfaces", len(t.links), "port", t.port)
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
				slog.Error("Read error", "iface", name, "err", err)
				continue
			}

			data := make([]byte, n)
			copy(data, buf[:n])

			t.msgs <- msg{data: data, addr: addr, iface: name}
		}
	}
}

func (t *Socket) ProcessMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-t.msgs:
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

	switch h.MsgType {
	case protocol.HELLOMsgType:
		var hello protocol.HELLO
		if err := hello.UnmarshalBinary(m.data); err != nil {
			slog.Error("Failed to unamrshal HELLO", "error", err)
			return
		}
		// slog.Debug(fmt.Sprintf("✅ [RECEIVED] HELLO from node %d | IP: %s | Port: %d | From: %s\n",
		// 	hello.SrcID, hello.SrcIP, hello.Port, m.addr))
		t.handleHELLO(&hello, m.addr)

	case protocol.RREQMsgType:
		var rreq protocol.RREQ
		if err := rreq.UnmarshalBinary(m.data); err != nil {
			slog.Error("Failed to unmarshal RREQ", "error", err)
			return
		}
		fmt.Printf("✅ [RECEIVED] RREQ from node %d | IP: %s | From: %s\n",
			rreq.SrcID, rreq.SrcIP, m.addr)
		t.handleRREQ(&rreq, m.addr, m.iface)

	case protocol.RREPMsgType:
		var rrep protocol.RREP
		if err := rrep.UnmarshalBinary(m.data); err != nil {
			slog.Error("Failed to unmarshal RREP", "error", err)
			return
		}
		fmt.Printf("✅ [RECEIVED] RREP from node %d | IP: %s | From: %s\n",
			rrep.SrcID, rrep.SrcIP, m.addr)
		t.handleRREP(&rrep, m.addr.AddrPort())

	case protocol.RERRMsgType:
		var rerr protocol.RERR
		if err := rerr.UnmarshalBinary(m.data); err != nil {
			slog.Error("Failed to unmarshal RERR", "error", err)
			return
		}
		fmt.Printf("✅ [RECEIVED] RERR from node %d | IP: %s | From: %s\n",
			rerr.SrcID, rerr.SrcIP, m.addr)

	case protocol.DATAMsgType:
		var data protocol.DATA
		if err := data.UnmarshalBinary(m.data); err != nil {
			slog.Error("Failed to unmarshal DATA", "error", err)
			return
		}
		fmt.Printf("✅ [RECEIVED] DATA from node %d | IP: %s | From: %s\n",
			data.SrcID, data.SrcIP, m.addr)
		t.handleDATA(&data)

	default:
		fmt.Printf("📦 [RECEIVED] Unknown message type %d from %s\n", h.MsgType, m.addr)
	}
}

func (t *Socket) handleHELLO(msg *protocol.HELLO, from *net.UDPAddr) {
	addrPort := netip.AddrPortFrom(
		netip.MustParseAddr(from.IP.String()),
		uint16(from.Port),
	)

	// slog.Debug("Updating neighbour",
	// 	"id", msg.SrcID,
	// 	"addr", addrPort,
	// 	"from_packet_src_ip", msg.SrcIP,
	// )

	routing.UpdateNeighbour(msg.SrcID, addrPort)
}

func (t *Socket) Broadcast(data []byte) {
	for name, ifaceData := range t.links {
		slog.Debug("Broadcast started", "interface", name)
		addr := &net.UDPAddr{
			IP:   net.IPv4bcast,
			Port: int(t.port),
		}
		_, err := ifaceData.conn.WriteToUDP(data, addr)
		if err != nil {
			slog.Error("Broadcast failed", "interface", name, "error", err)
		} else {
			slog.Debug("Broadcast complete", "interface", name, "len", len(data))
		}
	}
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
			slog.Error("Failed to create HELLO", "iface", name, "err", err)
			continue
		}

		data, err := hello.MarshalBinary()
		if err != nil {
			slog.Error("Failed to marshal HELLO", "iface", name, "err", err)
			continue
		}
		bcastAddr := &net.UDPAddr{
			IP:   net.IPv4bcast,
			Port: int(t.port),
		}

		_, err = link.conn.WriteToUDP(data, bcastAddr)
		if err != nil {
			slog.Warn("HELLO broadcast failed", "iface", name, "err", err)
		} else {
			// slog.Debug("HELLO broadcast sent", "iface", name, "src_ip", srcIP, "dst_ip", bcastAddr.IP)
		}
	}

	return nil
}

func (t *Socket) StartHelloSender(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	slog.Info("Hello sender started")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := t.broadcastHello(); err != nil {
				slog.Error("broadcastHello failed", "err", err)
			}
		}
	}
}
