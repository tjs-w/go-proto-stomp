package stomp

import (
	"errors"
	"fmt"
	"testing"
)

func TestTx(t *testing.T) {
	cleanupTx()

	txID := "tx"
	if err := startTx(txID); err != nil {
		t.Error(err)
	}

	out := []string{"Hello", "World"}
	if err := bufferTxMessage(txID, NewFrame(CmdMessage, nil, []byte(out[0]))); err != nil {
		t.Error(err)
	}

	if err := bufferTxMessage(txID, NewFrame(CmdMessage, nil, []byte(out[1]))); err != nil {
		t.Error(err)
	}

	m := 0
	if err := foreachTx(txID, func(frame *Frame) error {
		if out[m] != string(frame.body) {
			return fmt.Errorf("expected: %s, got: %s", out[m], string(frame.body))
		}
		m++
		return nil
	}); err != nil {
		t.Error(err)
	}

	if len(txBuffer) != 1 {
		t.Error(txBuffer)
	}

	if len(txBuffer[txID]) != 2 {
		t.Error(txBuffer)
	}

	if err := dropTx(txID); err != nil {
		t.Error(err)
	}

	if len(txBuffer) != 0 {
		t.Error(txBuffer)
	}
}

func TestTxErr(t *testing.T) {
	cleanupTx()

	if err := startTx(""); err == nil {
		t.Error()
	}

	if err := startTx("tx"); err != nil {
		t.Error()
	}

	if err := startTx("tx"); err == nil {
		t.Error()
	}

	if err := bufferTxMessage("", nil); err == nil {
		t.Error()
	}

	if err := bufferTxMessage("tx", NewFrame(CmdMessage, nil, []byte("Hello"))); err != nil {
		t.Error(err)
	}

	if err := foreachTx("tx", func(frame *Frame) error {
		return errors.New("tx err")
	}); err == nil {
		t.Error()
	}

	if err := foreachTx("", func(frame *Frame) error {
		return nil
	}); err == nil {
		t.Error()
	}

	if err := foreachTx("tx100", func(frame *Frame) error {
		return nil
	}); err == nil {
		t.Error()
	}

	if err := dropTx(""); err == nil {
		t.Error()
	}
	if err := dropTx("tx100"); err == nil {
		t.Error()
	}
}

func cleanupTx() {
	for k := range txBuffer {
		delete(txBuffer, k)
	}
}
