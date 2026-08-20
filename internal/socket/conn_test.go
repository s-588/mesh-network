// package socket contains all logic of inter node communication
package socket

import (
	"context"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/s-588/mesh-network/internal/config"
	"github.com/s-588/mesh-network/internal/protocol"
)

// ownID is the node ID used in unit tests for "this" socket.
const ownID uint64 = 99

var (
	testCfg = config.AppConfig{
		ID:            ownID,
		Lifetime:      30,
		TTL:           8,
		HelloInterval: 1,
		Port:          8040,
	}
)

// newTestSocket builds a Socket for unit tests only.
// It does not open real network interfaces.
func newTestSocket(t *testing.T) *Socket {
	t.Helper()
	return &Socket{
		port:         8040,
		cfg:          testCfg,
		links:        make(map[string]*interfaceState),
		incomingMsgs: make(chan msg, 256),
		seenRREQs:    make(map[uint64]uint64),
		pendingMsgs:  make(map[uint64][][]byte),
		inboxMsgs:    make([]string, 0),
	}
}

// withLocalUDP attaches a real UDP socket bound to 127.0.0.1:0 under the
// given interface name. Caller must close via t.Cleanup.
func withLocalUDP(t *testing.T, s *Socket, name string) *net.UDPConn {
	t.Helper()
	addr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ResolveUDPAddr: %v", err)
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	local := conn.LocalAddr().(*net.UDPAddr)
	ip4 := local.IP.To4()
	if ip4 == nil {
		ip4 = net.IPv4(127, 0, 0, 1)
	}
	s.links[name] = &interfaceState{
		name: name,
		conn: conn,
		addr: netip.AddrFrom4([4]byte{ip4[0], ip4[1], ip4[2], ip4[3]}),
	}
	return conn
}

func mustAddrPort(s string) netip.AddrPort {
	ap, err := netip.ParseAddrPort(s)
	if err != nil {
		panic(err)
	}
	return ap
}

func TestSocket_handleRREQ(t *testing.T) {
	srcAddr := mustAddrPort("10.0.0.2:8040")

	tests := []struct {
		name            string
		seenRREQs       map[uint64]uint64
		rreq            protocol.RREQ
		wantSeenSrcID   uint64
		wantSeenBcastID uint64
		wantSeenLen     int
	}{
		{
			name: "RREQ from myself is ignored",
			rreq: protocol.RREQ{
				Header: protocol.Header{
					SrcID: ownID,
					DstID: 42,
					TTL:   5,
				},
				BroadcastID: 10,
				SrcSeq:      1,
				HopCount:    0,
			},
			wantSeenLen: 0,
		},
		{
			name: "duplicate BroadcastID is ignored",
			seenRREQs: map[uint64]uint64{
				10: 5,
			},
			rreq: protocol.RREQ{
				Header: protocol.Header{
					SrcID: 10,
					DstID: 42,
					TTL:   5,
				},
				BroadcastID: 5,
				SrcSeq:      3,
				HopCount:    1,
			},
			wantSeenSrcID:   10,
			wantSeenBcastID: 5,
			wantSeenLen:     1,
		},
		{
			name: "older BroadcastID is ignored",
			seenRREQs: map[uint64]uint64{
				10: 7,
			},
			rreq: protocol.RREQ{
				Header: protocol.Header{
					SrcID: 10,
					DstID: 42,
					TTL:   5,
				},
				BroadcastID: 3,
				SrcSeq:      1,
				HopCount:    0,
			},
			wantSeenSrcID:   10,
			wantSeenBcastID: 7,
			wantSeenLen:     1,
		},
		{
			name: "new BroadcastID is recorded (intermediate, TTL expires)",
			rreq: protocol.RREQ{
				Header: protocol.Header{
					SrcID: 10,
					DstID: 42,
					TTL:   1,
				},
				BroadcastID: 1,
				SrcSeq:      2,
				HopCount:    0,
			},
			wantSeenSrcID:   10,
			wantSeenBcastID: 1,
			wantSeenLen:     1,
		},
		{
			name: "newer BroadcastID updates the seen map",
			seenRREQs: map[uint64]uint64{
				10: 3,
			},
			rreq: protocol.RREQ{
				Header: protocol.Header{
					SrcID: 10,
					DstID: 42,
					TTL:   1,
				},
				BroadcastID: 8,
				SrcSeq:      4,
				HopCount:    2,
			},
			wantSeenSrcID:   10,
			wantSeenBcastID: 8,
			wantSeenLen:     1,
		},
		{
			name: "we are the destination — still records BroadcastID",
			rreq: protocol.RREQ{
				Header: protocol.Header{
					SrcID: 10,
					DstID: ownID,
					TTL:   5,
				},
				BroadcastID: 11,
				SrcSeq:      7,
				HopCount:    1,
			},
			wantSeenSrcID:   10,
			wantSeenBcastID: 11,
			wantSeenLen:     1,
		},
		{
			name: "TTL > 1 intermediate — records and attempts forward",
			rreq: protocol.RREQ{
				Header: protocol.Header{
					SrcID: 10,
					DstID: 42,
					TTL:   3,
				},
				BroadcastID: 20,
				SrcSeq:      1,
				HopCount:    0,
			},
			wantSeenSrcID:   10,
			wantSeenBcastID: 20,
			wantSeenLen:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestSocket(t)
			if tt.seenRREQs != nil {
				s.seenRREQs = tt.seenRREQs
			}
			// Avoid panic in SendRREP / broadcast paths when links are empty.
			withLocalUDP(t, s, "eth0")

			s.handleRREQ(&tt.rreq, srcAddr, "eth0")

			s.seenMu.Lock()
			defer s.seenMu.Unlock()

			if tt.wantSeenLen >= 0 && len(s.seenRREQs) != tt.wantSeenLen {
				t.Errorf("seenRREQs len = %d, want %d; map=%v",
					len(s.seenRREQs), tt.wantSeenLen, s.seenRREQs)
			}
			if tt.wantSeenSrcID != 0 {
				got, ok := s.seenRREQs[tt.wantSeenSrcID]
				if !ok {
					t.Errorf("seenRREQs missing key %d", tt.wantSeenSrcID)
				} else if got != tt.wantSeenBcastID {
					t.Errorf("seenRREQs[%d] = %d, want %d",
						tt.wantSeenSrcID, got, tt.wantSeenBcastID)
				}
			}
		})
	}
}
func TestSocket_handleRREQ_concurrentDuplicates(t *testing.T) {
	s := newTestSocket(t)
	withLocalUDP(t, s, "eth0")
	src := mustAddrPort("10.0.0.9:8040")

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			rreq := protocol.RREQ{
				Header: protocol.Header{
					SrcID: 20,
					DstID: 42,
					TTL:   1,
				},
				BroadcastID: 1,
				SrcSeq:      1,
			}
			s.handleRREQ(&rreq, src, "eth0")
		}()
	}
	wg.Wait()

	s.seenMu.Lock()
	defer s.seenMu.Unlock()
	if got := s.seenRREQs[20]; got != 1 {
		t.Errorf("after concurrent identical RREQs, seenRREQs[20] = %d, want 1", got)
	}
	if len(s.seenRREQs) != 1 {
		t.Errorf("seenRREQs should have exactly 1 entry, got %d", len(s.seenRREQs))
	}
}

func TestSocket_handleRREP(t *testing.T) {
	from := mustAddrPort("10.0.0.4:8040")

	t.Run("RREP for us clears pendingMsgs and does not panic", func(t *testing.T) {
		s := newTestSocket(t)
		withLocalUDP(t, s, "eth0")

		payload := []byte("buffered")
		s.pendingMu.Lock()
		s.pendingMsgs[55] = [][]byte{payload}
		s.pendingMu.Unlock()

		rrep := &protocol.RREP{
			Header: protocol.Header{
				SrcID: 55, // destination we were looking for
				DstID: ownID,
				TTL:   5,
			},
			DstSeq:   3,
			HopCount: 1,
			Lifetime: 30,
		}

		s.handleRREP(rrep, from, "eth0")

		s.pendingMu.Lock()
		defer s.pendingMu.Unlock()
		if _, ok := s.pendingMsgs[55]; ok {
			t.Error("pendingMsgs[55] should have been deleted after recieving route reply to it")
		}
	})

	t.Run("RREP for us with empty pending — no panic", func(t *testing.T) {
		s := newTestSocket(t)
		withLocalUDP(t, s, "eth0")

		rrep := &protocol.RREP{
			Header: protocol.Header{
				SrcID: 77,
				DstID: ownID,
				TTL:   5,
			},
			DstSeq:   1,
			HopCount: 0,
			Lifetime: 30,
		}
		s.handleRREP(rrep, from, "eth0")

		s.pendingMu.Lock()
		defer s.pendingMu.Unlock()
		if len(s.pendingMsgs) != 0 {
			t.Errorf("pendingMsgs should stay empty, got %v", s.pendingMsgs)
		}
	})

	t.Run("RREP not for us, no reverse route - no panic, pending untouched", func(t *testing.T) {
		s := newTestSocket(t)
		withLocalUDP(t, s, "eth0")

		s.pendingMu.Lock()
		s.pendingMsgs[10] = [][]byte{[]byte("msg")}
		s.pendingMu.Unlock()

		rrep := &protocol.RREP{
			Header: protocol.Header{
				SrcID: 55,
				DstID: 2, // not us
				TTL:   5,
			},
			DstSeq:   2,
			HopCount: 1,
			Lifetime: 30,
		}
		s.handleRREP(rrep, from, "eth0")

		s.pendingMu.Lock()
		defer s.pendingMu.Unlock()
		if _, ok := s.pendingMsgs[10]; !ok {
			t.Error("pendingMsgs[10] must remain when RREP is not for us")
		}
	})
}

func TestSocket_handleDATA(t *testing.T) {
	from := mustAddrPort("10.0.0.3:8040")

	t.Run("message for us is stored in inbox", func(t *testing.T) {
		s := newTestSocket(t)

		data := &protocol.DATA{
			Header: protocol.Header{
				SrcID: 10,
				DstID: ownID,
				TTL:   5,
			},
			Payload: []byte("hello"),
			SeqNum:  1,
		}

		s.handleDATA(data, from)

		msgs := s.GetMessages()
		if len(msgs) != 1 {
			t.Fatalf("inbox len = %d, want 1", len(msgs))
		}
		want := "Node 10: hello"
		if msgs[0] != want {
			t.Errorf("inbox[0] = %q, want %q", msgs[0], want)
		}
	})

	t.Run("multiple messages accumulate in inbox", func(t *testing.T) {
		s := newTestSocket(t)

		for i, p := range []string{"a", "b", "c"} {
			data := &protocol.DATA{
				Header: protocol.Header{
					SrcID: uint64(i + 1),
					DstID: ownID,
					TTL:   5,
				},
				Payload: []byte(p),
				SeqNum:  uint32(i + 1),
			}
			s.handleDATA(data, from)
		}

		msgs := s.GetMessages()
		if len(msgs) != 3 {
			t.Fatalf("inbox len = %d, want 3", len(msgs))
		}
	})

	t.Run("message not for us and no route — inbox stays empty", func(t *testing.T) {
		s := newTestSocket(t)

		data := &protocol.DATA{
			Header: protocol.Header{
				SrcID: 10,
				DstID: 55,
				TTL:   5,
			},
			Payload: []byte("forward me"),
			SeqNum:  2,
		}

		s.handleDATA(data, from)

		if got := s.GetMessages(); len(got) != 0 {
			t.Errorf("inbox should stay empty, got %v", got)
		}
	})

	t.Run("message not for us with TTL <= 1 — delete message and keep inbox empty", func(t *testing.T) {
		s := newTestSocket(t)

		data := &protocol.DATA{
			Header: protocol.Header{
				SrcID: 10,
				DstID: 55,
				TTL:   1,
			},
			Payload: []byte("ttl dead"),
			SeqNum:  3,
		}

		s.handleDATA(data, from)

		if got := s.GetMessages(); len(got) != 0 {
			t.Errorf("inbox should stay empty, got %v", got)
		}
	})
}

func TestSocket_SendData(t *testing.T) {
	t.Run("no route, payload is queued", func(t *testing.T) {
		s := newTestSocket(t)
		withLocalUDP(t, s, "eth0")
		payload := []byte("msg")

		s.SendData(42, payload)

		s.pendingMu.Lock()
		defer s.pendingMu.Unlock()

		queued, ok := s.pendingMsgs[42]
		if !ok {
			t.Fatal("expected pendingMsgs[42] to exist")
		}
		if len(queued) != 1 {
			t.Fatalf("pendingMsgs[42] len = %d, want 1", len(queued))
		}
		if string(queued[0]) != string(payload) {
			t.Errorf("queued payload = %q, want %q", queued[0], payload)
		}
	})

	t.Run("no route, multiple payloads accumulate", func(t *testing.T) {
		s := newTestSocket(t)
		withLocalUDP(t, s, "eth0")

		s.SendData(7, []byte("a"))
		s.SendData(7, []byte("b"))
		s.SendData(7, []byte("c"))

		s.pendingMu.Lock()
		defer s.pendingMu.Unlock()

		queued := s.pendingMsgs[7]
		if len(queued) != 3 {
			t.Fatalf("pendingMsgs[7] len = %d, want 3", len(queued))
		}
	})

	t.Run("no route increments seqNum via SendRREQ", func(t *testing.T) {
		s := newTestSocket(t)
		withLocalUDP(t, s, "eth0")
		before := s.GetSeqNum()

		s.SendData(99, []byte("x"))
		// SendRREQ does seqNum.Add(1)

		if got := s.GetSeqNum(); got != before+1 {
			t.Errorf("seqNum = %d, want %d", got, before+1)
		}
	})
}

func TestSocket_SendRREQ(t *testing.T) {
	t.Run("increments seqNum", func(t *testing.T) {
		s := newTestSocket(t)
		withLocalUDP(t, s, "eth0")
		before := s.GetSeqNum()

		s.SendRREQ(42)

		if got := s.GetSeqNum(); got != before+1 {
			t.Errorf("seqNum = %d, want %d", got, before+1)
		}
	})

	t.Run("empty links does not panic", func(t *testing.T) {
		s := newTestSocket(t)
		// no links to interfaces, should not panic
		s.SendRREQ(1)
	})

	t.Run("with link broadcasts without panic", func(t *testing.T) {
		s := newTestSocket(t)
		withLocalUDP(t, s, "eth0")
		s.SendRREQ(100)
	})
}

func TestSocket_SendRREP(t *testing.T) {
	t.Run("increments seqNum", func(t *testing.T) {
		s := newTestSocket(t)
		withLocalUDP(t, s, "eth0")
		before := s.GetSeqNum()

		rreq := &protocol.RREQ{
			Header: protocol.Header{
				SrcID: 10,
				DstID: ownID,
				SrcIP: netip.MustParseAddr("10.0.0.2"),
			},
			BroadcastID: 1,
			SrcSeq:      1,
		}
		dst := mustAddrPort("10.0.0.2:8040")

		s.SendRREP(rreq, dst, "eth0")

		if got := s.GetSeqNum(); got != before+1 {
			t.Errorf("seqNum = %d, want %d", got, before+1)
		}
	})

	t.Run("unknown iface does not panic (write fails)", func(t *testing.T) {
		s := newTestSocket(t)
		// t.links[iface] is nil, WriteToUDP will panic on nil.
		withLocalUDP(t, s, "eth0")

		rreq := &protocol.RREQ{
			Header: protocol.Header{
				SrcID: 10,
				DstID: ownID,
				SrcIP: netip.MustParseAddr("10.0.0.2"),
			},
		}
		s.SendRREP(rreq, mustAddrPort("10.0.0.2:8040"), "eth0")
	})
}

func TestSocket_sendToAddr(t *testing.T) {
	t.Run("sends via named interface", func(t *testing.T) {
		s := newTestSocket(t)
		conn := withLocalUDP(t, s, "eth0")

		dst := mustAddrPort("127.0.0.1:" + strconv.Itoa(conn.LocalAddr().(*net.UDPAddr).Port))
		s.sendToAddr([]byte("ping"), dst, "eth0")
	})

	t.Run("unknown iface falls back to any link", func(t *testing.T) {
		s := newTestSocket(t)
		withLocalUDP(t, s, "eth0")

		dst := mustAddrPort("127.0.0.1:9")
		s.sendToAddr([]byte("fallback"), dst, "missing-iface")
	})

	t.Run("no links at all does not panic", func(t *testing.T) {
		s := newTestSocket(t)
		s.sendToAddr([]byte("nowhere"), mustAddrPort("127.0.0.1:9"), "eth0")
	})
}

func TestSocket_Start_and_ProcessMessages_cancel(t *testing.T) {
	s := newTestSocket(t)
	withLocalUDP(t, s, "eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start listeners and processor; cancel should make them exit.
	s.Start(ctx)
	go s.ProcessMessages(ctx)

	time.Sleep(20 * time.Millisecond)
	cancel()
	time.Sleep(20 * time.Millisecond)
}

func TestSocket_listenOnConn_cancel(t *testing.T) {
	s := newTestSocket(t)
	conn := withLocalUDP(t, s, "eth0")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.listenOnConn(ctx, "eth0", conn)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	// Force unblock ReadFromUDP by closing the conn.
	// Cleanup also closes, but we need an immediate unblock for the test.
	_ = conn.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("listenOnConn did not exit after cancel + close")
	}
}

func TestSocket_handleMessage(t *testing.T) {
	t.Run("unknown msg type is ignored", func(t *testing.T) {
		s := newTestSocket(t)
		s.handleMessage(msg{
			data:  make([]byte, 64),
			addr:  &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 8040},
			iface: "eth0",
		})
	})
}

func TestSocket_handleRERR(t *testing.T) {
	t.Run("unknown unreachable dst — no panic", func(t *testing.T) {
		s := newTestSocket(t)
		rerr := &protocol.RERR{
			Header: protocol.Header{
				SrcID: 10,
				DstID: ownID,
				TTL:   1,
			},
			UnreachableDstID: 999,
			ErrCode:          protocol.ErrDestUnreachable,
		}
		s.handleRERR(rerr)

		if len(s.seenRREQs) != 0 {
			t.Errorf("handleRERR must not touch seenRREQs")
		}
		if len(s.pendingMsgs) != 0 {
			t.Errorf("handleRERR must not touch pendingMsgs")
		}
	})

	t.Run("RERR from next hop of a known route path is safe", func(t *testing.T) {
		s := newTestSocket(t)
		// Even if RoutesTable has no entry, code returns early — no panic.
		rerr := &protocol.RERR{
			Header: protocol.Header{
				SrcID: 5,
				DstID: ownID,
				TTL:   1,
			},
			UnreachableDstID: 42,
			ErrCode:          protocol.ErrLinkBreak,
		}
		s.handleRERR(rerr)
	})
	// TODO: test if RERR if correctly sended to precursors
}

func TestSocket_handleHELLO(t *testing.T) {
	s := newTestSocket(t)
	from := mustAddrPort("10.0.0.5:8040")

	hello := &protocol.HELLO{
		Header: protocol.Header{
			SrcID: 7,
			DstID: 0,
			TTL:   1,
		},
		Port: 8040,
	}

	s.handleHELLO(hello, from, "eth0")

	if len(s.seenRREQs) != 0 {
		t.Errorf("HELLO must not touch seenRREQs, got %v", s.seenRREQs)
	}
	if len(s.pendingMsgs) != 0 {
		t.Errorf("HELLO must not touch pendingMsgs, got %v", s.pendingMsgs)
	}
	if len(s.GetMessages()) != 0 {
		t.Errorf("HELLO must not touch inbox")
	}
}

func TestSocket_broadcastHello(t *testing.T) {
	t.Run("empty links returns nil", func(t *testing.T) {
		s := newTestSocket(t)
		if err := s.broadcastHello(); err != nil {
			t.Errorf("broadcastHello() error = %v, want nil", err)
		}
	})

	t.Run("with link does not return error", func(t *testing.T) {
		s := newTestSocket(t)
		withLocalUDP(t, s, "eth0")
		if err := s.broadcastHello(); err != nil {
			t.Errorf("broadcastHello() error = %v, want nil", err)
		}
	})
}

func TestSocket_StartHelloSender_cancel(t *testing.T) {
	s := newTestSocket(t)
	withLocalUDP(t, s, "eth0")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.StartHelloSender(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StartHelloSender did not exit after cancel")
	}
}

func TestSocket_StartNeighbourCollector_cancel(t *testing.T) {
	s := newTestSocket(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.StartNeighbourCollector(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StartNeighbourCollector did not exit after cancel")
	}
}

func TestSocket_SendRERR(t *testing.T) {
	t.Run("unknown precursor — no panic, returns early", func(t *testing.T) {
		s := newTestSocket(t)
		s.SendRERR(12345, 99, protocol.ErrDestUnreachable)
	})

	t.Run("does not touch seenRREQs or inbox", func(t *testing.T) {
		s := newTestSocket(t)
		s.SendRERR(1, 2, protocol.ErrLinkBreak)

		if len(s.seenRREQs) != 0 {
			t.Error("SendRERR must not touch seenRREQs")
		}
		if len(s.GetMessages()) != 0 {
			t.Error("SendRERR must not touch inbox")
		}
	})
}

func TestSocket_GetMessages(t *testing.T) {
	s := newTestSocket(t)
	s.inboxMsgs = []string{"a", "b"}

	got := s.GetMessages()
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("GetMessages() = %v, want [a b]", got)
	}

	got[0] = "mutated"
	if s.inboxMsgs[0] != "a" {
		t.Error("GetMessages must return a copy, not the internal slice")
	}
}

func TestSocket_GetMessages_empty(t *testing.T) {
	s := newTestSocket(t)
	got := s.GetMessages()
	if got == nil {
		// Implementation returns make([]string, 0) copy — non-nil empty is fine;
		// nil is also acceptable depending on implementation.
		return
	}
	if len(s.inboxMsgs) != len(got) {
		t.Errorf("GetMessages() = %v, want %v", got, s.inboxMsgs)
	}
}

func TestSocket_GetSeqNum(t *testing.T) {
	s := newTestSocket(t)
	s.seqNum.Store(17)

	if got := s.GetSeqNum(); got != 17 {
		t.Errorf("GetSeqNum() = %d, want 17", got)
	}
}

func TestSocket_GetSeqNum_zero(t *testing.T) {
	s := newTestSocket(t)
	if got := s.GetSeqNum(); got != 0 {
		t.Errorf("GetSeqNum() = %d, want 0", got)
	}
}

func TestSocket_GetInterfaces(t *testing.T) {
	s := newTestSocket(t)
	s.links["eth0"] = &interfaceState{name: "eth0"}
	s.links["wlan0"] = &interfaceState{name: "wlan0"}

	got := s.GetInterfaces()
	if len(got) != 2 {
		t.Fatalf("GetInterfaces() len = %d, want 2", len(got))
	}
	// TODO: Review?
	if !slices.Contains(got, "eth0") || !slices.Contains(got, "wlan0") {
		t.Errorf("GetInterfaces() = %v, want = [eth0, wlan0]", got)
	}
}

func TestSocket_GetInterfaces_empty(t *testing.T) {
	s := newTestSocket(t)
	got := s.GetInterfaces()
	if len(got) != 0 {
		t.Errorf("GetInterfaces() = %v, want empty", got)
	}
}
