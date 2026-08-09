package cli

import (
	"errors"
	"strconv"
)

var (
	incorrectDstErr = errors.New("incorrect destination ID")
	emptyMsgErr     = errors.New("message payload must not be empty")
)

// checkDst check if destination is match the requirements.
func checkDst(dst string) error {
	if dst == "" {
		return incorrectDstErr
	}
	if num, err := strconv.ParseInt(dst, 10, 64); err != nil || num < 0 {
		return incorrectDstErr
	}
	return nil
}

// checkMsg check if message is match the requirements.
func checkMsg(msg string) error {
	if msg == "" {
		return emptyMsgErr
	}
	return nil
}
