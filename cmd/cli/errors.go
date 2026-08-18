package cli

import (
	"errors"
	"strconv"
)

var (
	errIncorrectDst = errors.New("incorrect destination ID")
	errEmptyMsg     = errors.New("message payload must not be empty")
)

// checkDst check if destination is match the requirements.
func checkDst(dst string) error {
	if dst == "" {
		return errIncorrectDst
	}
	if num, err := strconv.ParseInt(dst, 10, 64); err != nil || num < 0 {
		return errIncorrectDst
	}
	return nil
}

// checkMsg check if message is match the requirements.
func checkMsg(msg string) error {
	if msg == "" {
		return errEmptyMsg
	}
	return nil
}
