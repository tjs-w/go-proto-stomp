package stomp

import (
	"fmt"
)

type stompErrorType string

const (
	errByteFormat         stompErrorType = "Invalid wire format"
	errProtocolFrame      stompErrorType = "Invalid frame format"
	errNetwork            stompErrorType = "Network error"
	errInvalidArg         stompErrorType = "Invalid argument"
	errFrameScanner       stompErrorType = "Frame scanning error"
	errBrokerStateMachine stompErrorType = "Protocol (broker) state-machine error"
	errTransaction        stompErrorType = "Transaction error"
)

func errorMsg(t stompErrorType, msg string) error {
	// s := string(debug.Stack())
	return fmt.Errorf("stomp: %s: %s", t, msg)
}
