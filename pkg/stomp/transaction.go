package stomp

import "fmt"

// txBuffer stores the queued-up message-frames for the transactions
// It is the mapping from the transaction ID to list of messages
var txBuffer = map[string][]*Frame{}

// startTx begins the transaction by creating the buffer queue for the txID
func startTx(txID string) error {
	if txID == "" {
		return errorMsg(errTransaction, "Missing transaction ID")
	}
	if _, ok := txBuffer[txID]; ok {
		return errorMsg(errTransaction, "Transaction already began/present (possible duplicate), TxID: "+txID)
	}
	txBuffer[txID] = []*Frame{}
	return nil
}

// bufferTxMessage adds the message to the transaction queue
func bufferTxMessage(txID string, msgFrame *Frame) error {
	if _, ok := txBuffer[txID]; !ok {
		return errorMsg(errTransaction, "No such transaction present, TxID: "+txID)
	}
	txBuffer[txID] = append(txBuffer[txID], msgFrame)
	return nil
}

// foreachTx executes the closure on each message in the list for given transaction
func foreachTx(txID string, fn func(*Frame) error) error {
	if txID == "" {
		return errorMsg(errTransaction, "Missing transaction ID when committing")
	}
	if _, ok := txBuffer[txID]; !ok {
		return errorMsg(errTransaction, fmt.Sprintf("Transaction ID '%s' not found in txBuffer", txID))
	}
	for _, frame := range txBuffer[txID] {
		if err := fn(frame); err != nil {
			return err
		}
	}
	return nil
}

// dropTx removes the transaction messages from the txBuffer
func dropTx(txID string) error {
	if txID == "" {
		return errorMsg(errTransaction, "Missing transaction ID when cancelling")
	}
	if _, ok := txBuffer[txID]; !ok {
		return errorMsg(errTransaction, fmt.Sprintf("Transaction ID '%s' not found for deletion from txBuffer", txID))
	}
	delete(txBuffer, txID)
	return nil
}
