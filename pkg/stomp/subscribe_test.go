package stomp

import (
	"testing"
)

func Test_readAck(t *testing.T) {
	dest, subsID, ackNum, err := scanAckNum(fmtAckNum("/queue/x", "3f9e7c9deb0a", 10))
	if err != nil {
		t.Error(err)
	}
	if dest != "/queue/x" || subsID != "3f9e7c9deb0a" || ackNum != 10 {
		t.Error(dest, subsID, ackNum)
	}
}
