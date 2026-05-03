package protocol_test

import (
	"net/netip"
	"reflect"
	"testing"

	"github.com/s-588/mesh-network/internal/protocol"
)

func TestMsgType_IsValid(t *testing.T) {
	data := []protocol.MsgType{
		protocol.HELLOMsgType,
		protocol.RREQMsgType,
		protocol.RREPMsgType,
		protocol.RERRMsgType,
		protocol.DATAMsgType,
	}
	for _, msgType := range data {
		if !msgType.IsValid() {
			t.Errorf("message type %v is invalid, but must be valid", msgType)
		}
	}
}

func TestNewHeader(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		opts    protocol.HeaderOpts
		want    *protocol.Header
		wantErr bool
	}{
		struct {
			name    string
			opts    protocol.HeaderOpts
			want    *protocol.Header
			wantErr bool
		}{
			name:    "empty opts, want err",
			opts:    protocol.HeaderOpts{},
			want:    nil,
			wantErr: true,
		},
		struct {
			name    string
			opts    protocol.HeaderOpts
			want    *protocol.Header
			wantErr bool
		}{
			name: "normal header",
			opts: protocol.HeaderOpts{
				MsgType:   protocol.HELLOMsgType,
				SrcIP:     netip.AddrFrom4([4]byte{128, 0, 0, 1}),
				DstIP:     netip.AddrFrom4([4]byte{128, 0, 0, 1}),
				SrcID:     1,
				DstID:     2,
				Timestamp: 3,
				TTL:       4,
			},
			want: &protocol.Header{
				MsgType:   protocol.HELLOMsgType,
				SrcIP:     netip.AddrFrom4([4]byte{128, 0, 0, 1}),
				DstIP:     netip.AddrFrom4([4]byte{128, 0, 0, 1}),
				SrcID:     1,
				DstID:     2,
				Timestamp: 3,
				TTL:       4,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := protocol.NewHeader(tt.opts)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("NewHeader() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Errorf("NewHeader() succeeded unexpectedly:\ngot %v\nwant %v\nwant err = %t", got, tt.want, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewHeader() failed:\ngot: %v\nwant: %v", got, tt.want)
			}
		})
	}
}

func TestHeader_RoundTrip(t *testing.T) {
	h, err := protocol.NewHeader(protocol.HeaderOpts{
		MsgType:   protocol.HELLOMsgType,
		SrcIP:     netip.AddrFrom4([4]byte{127, 0, 0, 1}),
		DstIP:     netip.AddrFrom4([4]byte{127, 0, 0, 1}),
		SrcID:     1,
		DstID:     2,
		Timestamp: 3,
		TTL:       4,
	})
	if err != nil {
		t.Fatalf("NewHeader unexpected fail: %s", err)
	}

	data, err := h.MarshalBinary()
	if err != nil {
		t.Fatalf("Marshal unexpected fail: %s", err)
	}

	newH := &protocol.Header{}
	err = newH.UnmarshalBinary(data)
	if err != nil {
		t.Fatalf("Unmarshal unexpected fail: %s", err)
	}

	if !reflect.DeepEqual(h, newH) {
		t.Fatalf("round trip failed:\nwant:%v\ngot:%v", h, newH)
	}

}

func TestNewHELLO(t *testing.T) {
	tests := []struct {
		name    string
		opts    protocol.HELLOOpts
		want    *protocol.HELLO
		wantErr bool
	}{
		{
			name:    "empty opts, want err",
			opts:    protocol.HELLOOpts{}, 
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid SrcIP (not IPv4), want err",
			opts: protocol.HELLOOpts{
				HeaderOpts: protocol.HeaderOpts{
					MsgType:   protocol.HELLOMsgType,
					SrcIP:     netip.MustParseAddr("::1"), 
					DstIP:     netip.AddrFrom4([4]byte{128, 0, 0, 1}),
					SrcID:     1,
					DstID:     2,
					Timestamp: 3,
					TTL:       4,
				},
				Port: 5001,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid DstIP (not IPv4), want err",
			opts: protocol.HELLOOpts{
				HeaderOpts: protocol.HeaderOpts{
					MsgType:   protocol.HELLOMsgType,
					SrcIP:     netip.AddrFrom4([4]byte{128, 0, 0, 1}),
					DstIP:     netip.MustParseAddr("::1"), 
					SrcID:     1,
					DstID:     2,
					Timestamp: 3,
					TTL:       4,
				},
				Port: 5001,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "zero Port, want err",
			opts: protocol.HELLOOpts{
				HeaderOpts: protocol.HeaderOpts{
					MsgType:   protocol.HELLOMsgType,
					SrcIP:     netip.AddrFrom4([4]byte{128, 0, 0, 1}),
					DstIP:     netip.AddrFrom4([4]byte{128, 0, 0, 1}),
					SrcID:     1,
					DstID:     2,
					Timestamp: 3,
					TTL:       4,
				},
				Port: 0,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "normal hello",
			opts: protocol.HELLOOpts{
				HeaderOpts: protocol.HeaderOpts{
					MsgType:   protocol.HELLOMsgType,
					SrcIP:     netip.AddrFrom4([4]byte{128, 0, 0, 1}),
					DstIP:     netip.AddrFrom4([4]byte{128, 0, 0, 1}),
					SrcID:     1,
					DstID:     2,
					Timestamp: 3,
					TTL:       4,
				},
				Port: 5001,
			},
			want: &protocol.HELLO{
				Header: protocol.Header{
					MsgType:   protocol.HELLOMsgType,
					SrcIP:     netip.AddrFrom4([4]byte{128, 0, 0, 1}),
					DstIP:     netip.AddrFrom4([4]byte{128, 0, 0, 1}),
					SrcID:     1,
					DstID:     2,
					Timestamp: 3,
					TTL:       4,
				},
				Port: 5001,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := protocol.NewHELLO(tt.opts)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("NewHELLO() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Errorf("NewHELLO() succeeded unexpectedly:\ngot %v\nwant %v\nwant err = %t",
					got, tt.want, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewHELLO() failed:\ngot:  %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

func TestHELLO_RoundTrip(t *testing.T) {
	hello, err := protocol.NewHELLO(protocol.HELLOOpts{
		HeaderOpts: protocol.HeaderOpts{
			MsgType:   protocol.HELLOMsgType,
			SrcIP:     netip.AddrFrom4([4]byte{127, 0, 0, 1}),
			DstIP:     netip.AddrFrom4([4]byte{127, 0, 0, 1}),
			SrcID:     1,
			DstID:     2,
			Timestamp: 3,
			TTL:       4,
		},
		Port: 5001,
	})
	if err != nil {
		t.Fatalf("NewHELLO unexpected fail: %s", err)
	}

	data, err := hello.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary unexpected fail: %s", err)
	}

	if len(data) != protocol.HELLOSize {
		t.Fatalf("marshal size mismatch: got %d, want %d", len(data), protocol.HELLOSize)
	}

	newHello := &protocol.HELLO{}
	err = newHello.UnmarshalBinary(data)
	if err != nil {
		t.Fatalf("UnmarshalBinary unexpected fail: %s", err)
	}

	if !reflect.DeepEqual(hello, newHello) {
		t.Fatalf("round trip failed:\nwant:\n%+v\ngot:\n%+v", hello, newHello)
	}
}

func TestNewRREQ(t *testing.T) {
	tests := []struct {
		name    string
		opts    protocol.RREQOpts
		want    *protocol.RREQ
		wantErr bool
	}{
		{
			name:    "empty opts, want err",
			opts:    protocol.RREQOpts{},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid SrcIP (IPv6), want err",
			opts: protocol.RREQOpts{
				HeaderOpts: protocol.HeaderOpts{
					MsgType:   protocol.RREQMsgType,
					SrcIP:     netip.MustParseAddr("::1"), // IPv6
					DstIP:     netip.AddrFrom4([4]byte{127, 0, 0, 1}),
					SrcID:     1,
					DstID:     2,
					Timestamp: 3,
					TTL:       4,
				},
				SrcSeq:   100,
				DstSeq:   200,
				HopCount: 5,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid DstIP (IPv6), want err",
			opts: protocol.RREQOpts{
				HeaderOpts: protocol.HeaderOpts{
					MsgType:   protocol.RREQMsgType,
					SrcIP:     netip.AddrFrom4([4]byte{127, 0, 0, 1}),
					DstIP:     netip.MustParseAddr("::1"), // IPv6
					SrcID:     1,
					DstID:     2,
					Timestamp: 3,
					TTL:       4,
				},
				SrcSeq:   100,
				DstSeq:   200,
				HopCount: 5,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "normal RREQ",
			opts: protocol.RREQOpts{
				HeaderOpts: protocol.HeaderOpts{
					MsgType:   protocol.RREQMsgType,
					SrcIP:     netip.AddrFrom4([4]byte{127, 0, 0, 1}),
					DstIP:     netip.AddrFrom4([4]byte{127, 0, 0, 1}),
					SrcID:     1,
					DstID:     2,
					Timestamp: 3,
					TTL:       4,
				},
				SrcSeq:   100,
				DstSeq:   200,
				HopCount: 5,
			},
			want: &protocol.RREQ{
				Header: protocol.Header{
					MsgType:   protocol.RREQMsgType,
					SrcIP:     netip.AddrFrom4([4]byte{127, 0, 0, 1}),
					DstIP:     netip.AddrFrom4([4]byte{127, 0, 0, 1}),
					SrcID:     1,
					DstID:     2,
					Timestamp: 3,
					TTL:       4,
				},
				SrcSeq:      100,
				DstSeq:      200,
				HopCount:    5,
				BroadcastID: 300,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := protocol.NewRREQ(tt.opts)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("NewRREQ() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Errorf("NewRREQ() succeeded unexpectedly:\ngot %v\nwant %v\nwant err = %t",
					got, tt.want, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewRREQ() failed:\ngot:  %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

func TestRREQ_RoundTrip(t *testing.T) {
	rreq, err := protocol.NewRREQ(protocol.RREQOpts{
		HeaderOpts: protocol.HeaderOpts{
			MsgType:   protocol.RREQMsgType,
			SrcIP:     netip.AddrFrom4([4]byte{127, 0, 0, 1}),
			DstIP:     netip.AddrFrom4([4]byte{127, 0, 0, 1}),
			SrcID:     1,
			DstID:     2,
			Timestamp: 3,
			TTL:       4,
		},
		SrcSeq:   100,
		DstSeq:   200,
		HopCount: 5,
	})
	if err != nil {
		t.Fatalf("NewRREQ unexpected fail: %s", err)
	}

	data, err := rreq.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary unexpected fail: %s", err)
	}

	if len(data) != protocol.RREQSize {
		t.Fatalf("marshal size mismatch: got %d, want %d", len(data), protocol.RREQSize)
	}

	newRREQ := &protocol.RREQ{}
	err = newRREQ.UnmarshalBinary(data)
	if err != nil {
		t.Fatalf("UnmarshalBinary unexpected fail: %s", err)
	}

	if !reflect.DeepEqual(rreq, newRREQ) {
		t.Fatalf("round trip failed:\nwant:\n%+v\ngot:\n%+v", rreq, newRREQ)
	}
}
