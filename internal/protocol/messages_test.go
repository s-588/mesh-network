package protocol_test

import (
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
		struct{name string; opts protocol.HeaderOpts; want *protocol.Header; wantErr bool}{
			name: "empty opts, want err",
			opts: protocol.HeaderOpts{},
			want: nil,
			wantErr: true,
		},
		struct{name string; opts protocol.HeaderOpts; want *protocol.Header; wantErr bool}{
			name: "normal header",
			opts: protocol.HeaderOpts{
				MsgType: protocol.HELLOMsgType,
				SrcIP: netip.AddrFrom4([4]byte{128,0,0,1}),
				DstIP: netip.AddrFrom4([4]byte{128,0,0,1}),
				SrcID: 1,
				DstID: 2,
				Timestamp: 3,
				TTL: 4,
			},
			want: &protocol.Header{
				MsgType: protocol.HELLOMsgType,
				SrcIP: netip.AddrFrom4([4]byte{128,0,0,1}),
				DstIP: netip.AddrFrom4([4]byte{128,0,0,1}),
				SrcID: 1,
				DstID: 2,
				Timestamp: 3,
				TTL: 4,
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
				t.Errorf("NewHeader() succeeded unexpectedly:\ngot %v\nwant %v\nwant err = %t",got, tt.want, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want){
				t.Errorf("NewHeader() failed:\ngot: %v\nwant: %v", got, tt.want)
			}
		})
	}
}
