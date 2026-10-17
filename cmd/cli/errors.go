package cli

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxMsgLen is the largest user payload accepted by the CLI and the IPC server.
// It stays under a single UDP datagram and a localhost query string.
const MaxMsgLen = 1024

var (
	errEmptyMsg   = errors.New("message must not be empty or only whitespace")
	errMsgTooLong = fmt.Errorf("message is too long (max %d bytes)", MaxMsgLen)
	errMsgNotUTF8 = errors.New("message must be valid text")
)

// CheckMsg reports whether msg can be sent to another node.
func CheckMsg(msg string) error {
	if strings.TrimSpace(msg) == "" {
		return errEmptyMsg
	}
	if len(msg) > MaxMsgLen {
		return errMsgTooLong
	}
	if !utf8.ValidString(msg) {
		return errMsgNotUTF8
	}
	return nil
}
