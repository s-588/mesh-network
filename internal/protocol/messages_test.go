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
