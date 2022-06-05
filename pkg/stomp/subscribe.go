package stomp

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/RoaringBitmap/roaring"
	set "github.com/deckarep/golang-set"
)

type subsInfo struct {
	sync.Mutex

	sessionHandler   *SessionHandler
	ackMode          AckMode
	nextAckNum       uint32
	pendingAckBitmap roaring.Bitmap
}

// subsToInfo: SubscriptionID => (Session, AckMode[auto/client/client-individual], etc...)
type subsToInfo map[string]*subsInfo

var (
	// destToSubsMap: Destination => map{ SubscriptionID => SessionInfo }
	destToSubsMap = map[string]subsToInfo{}

	// sessToDestSubs: SessionID => []SubscriberID
	sessToSubsMap = map[string]set.Set{}

	// subsToDestMap: SubscriptionID => Destination
	subsToDestMap = map[string]string{}
)

func addSubscription(dest string, subsID string, ackMode AckMode, sess *SessionHandler) error {
	if _, ok := destToSubsMap[dest][subsID]; !ok {
		destToSubsMap[dest] = subsToInfo{}
	}

	destToSubsMap[dest][subsID] = &subsInfo{
		sessionHandler: sess,
		ackMode:        ackMode,
	}

	subsToDestMap[subsID] = dest

	if _, ok := sessToSubsMap[sess.sessionID]; !ok {
		sessToSubsMap[sess.sessionID] = set.NewSet()
	}
	sessToSubsMap[sess.sessionID].Add(subsID)
	return nil
}

func removeSubscription(subsID string) error {
	if _, ok := subsToDestMap[subsID]; !ok {
		return errorMsg(errBrokerStateMachine, "No such subscription present to unsubscribe, subsID: "+subsID)
	}
	dest := subsToDestMap[subsID]

	if _, ok := destToSubsMap[dest]; !ok {
		return errorMsg(errBrokerStateMachine, "No such subscription for given destination, subsID: "+subsID)
	}
	sess := destToSubsMap[dest][subsID].sessionHandler.sessionID

	sessToSubsMap[sess].Remove(subsID)
	delete(destToSubsMap[dest], subsID)
	delete(subsToDestMap, subsID)
	return nil
}

func cleanupSubscriptions(sessionID string) error {
	if _, ok := sessToSubsMap[sessionID]; !ok {
		return nil
	}
	for _, subsID := range sessToSubsMap[sessionID].ToSlice() {
		if err := removeSubscription(subsID.(string)); err != nil {
			return err
		}
	}
	delete(sessToSubsMap, sessionID)
	return nil
}

func publish(frame *Frame, txID string) error {
	dest := frame.headers[HdrKeyDestination]
	if _, ok := destToSubsMap[dest]; !ok {
		return errorMsg(errBrokerStateMachine, "Missing entry in destToSubsMap, for key: "+dest)
	}

	sendIt := func(subsID string, info *subsInfo, wg *sync.WaitGroup) {
		info.Lock()
		defer info.Unlock()

		if err := info.sessionHandler.sendMessage(dest, subsID, info.nextAckNum, txID,
			frame.headers, frame.body); err != nil {
			log.Println(err)
			return
		}
		info.pendingAckBitmap.Add(info.nextAckNum)
		info.nextAckNum++

		wg.Done()
	}

	var wg sync.WaitGroup
	for subsID, info := range destToSubsMap[dest] {
		wg.Add(1)
		if txID == "" {
			// Parallelize sending non-tx messages
			go sendIt(subsID, info, &wg)
		} else {
			// Send tx messages in same sequence (do not parallelize to maintain order)
			sendIt(subsID, info, &wg)
		}
	}
	wg.Wait()

	return nil
}

func fmtAckNum(dest, subsID string, ackNum uint32) string {
	return fmt.Sprintf("%s:%s:%d", dest, subsID, ackNum)
}

func scanAckNum(fmtAck string) (dest string, subsID string, ackNum uint32, err error) {
	parts := strings.Split(fmtAck, ":")
	var n int
	n, err = strconv.Atoi(parts[2])
	if err != nil {
		return "", "", 0, err
	}
	if n < 0 {
		err = errorMsg(errBrokerStateMachine, "Invalid ack value: "+fmtAck)
		return "", "", 0, err
	}
	return parts[0], parts[1], uint32(n), nil
}

// func processAck(ackVal string) error {
// 	dest, subsID, ackNum, err := scanAckNum(ackVal)
// 	if err != nil {
// 		return errorMsg(errBrokerStateMachine, "Invalid ACK value: "+ackVal)
// 	}
//
// 	if _, ok := destToSubsMap[dest]; !ok {
// 		return errorMsg(errBrokerStateMachine, "Missing entry in destToSubsMap, for key: "+dest)
// 	}
//
// 	if _, ok := destToSubsMap[dest][subsID]; !ok {
// 		return errorMsg(errBrokerStateMachine, "Missing entry in destToSubsMap, for key: "+dest+"/"+subsID)
// 	}
//
// 	info := destToSubsMap[dest][subsID]
// 	info.Lock()
// 	defer info.Unlock()
// 	if info.ackMode == HdrValAckClient {
// 		for i := info.pendingAckBitmap.Minimum(); info.pendingAckBitmap.Contains(i); i++ {
// 			info.pendingAckBitmap.Remove(i)
// 		}
// 	} else if info.ackMode == HdrValAckClientIndividual {
// 		info.pendingAckBitmap.Remove(ackNum)
// 	}
// 	return nil
// }
//
// func processNack(ackVal string) error {
// 	// Do nothing
// 	return nil
// }
