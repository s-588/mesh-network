package cli

import (
	"errors"
)

var (
	errIncorrectDst = errors.New("incorrect destination ID")
	errEmptyMsg     = errors.New("message payload must not be empty")
)

// checkDst check if destination is match the requirements.
func checkDst(dst int64) error {
	if dst < 0 {
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
