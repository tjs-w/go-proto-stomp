package stomp

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/cenkalti/backoff"
	"github.com/go-co-op/gocron"
	"github.com/google/uuid"
)

// Session handles the STOMP client session on connection
type Session struct {
	conn               net.Conn
	sessionID          string
	loginFunc          LoginFunc
	wgSessions         *sync.WaitGroup
	hbSendIntervalMsec int
	hbRecvIntervalMsec int
	hbJob              *gocron.Job
}

// NewSession creates a new session object & maintains the session state internally
func NewSession(conn net.Conn, loginFunc LoginFunc, wg *sync.WaitGroup,
	heartbeatSendIntervalMsec, heartbeatReceiveIntervalMsec int,
) *Session {
	return &Session{
		conn:               conn,
		loginFunc:          loginFunc,
		sessionID:          uuid.NewString(),
		wgSessions:         wg,
		hbSendIntervalMsec: heartbeatSendIntervalMsec,
		hbRecvIntervalMsec: heartbeatReceiveIntervalMsec,
	}
}

// LoginFunc represents the user-defined authentication function
type LoginFunc func(login, passcode string) error

// Start begins the STOMP session with the Client
func (sess *Session) Start() {
	defer sess.cleanup()
	for raw := range frameScanner(sess.conn) {
		frame, err := NewFrameFromBytes(raw)
		if err != nil {
			_ = sess.sendError(err, fmt.Sprint("Frame serialization error:"+frame.String()))
			return
		}
		if err = frame.Validate(ClientFrame); err != nil {
			_ = sess.sendError(err, fmt.Sprint("Frame validation error:"+frame.String()))
			return
		}

		if err = sess.stateMachine(frame); err != nil {
			log.Println(err)
			return
		}
	}
}

func (sess *Session) cleanup() {
	_ = sess.conn.Close()
	sess.wgSessions.Done()
	if sess.hbJob != nil {
		sched.RemoveByReference(sess.hbJob)
	}
}

// sendError is the helper function to send the ERROR frames
func (sess *Session) sendError(err error, payload string) error {
	return sess.send(CmdError, map[Header]string{
		HdrKeyContentType:   "text/plain",
		HdrKeyContentLength: strconv.Itoa(len(payload)),
		HdrKeyMessage:       err.Error(),
	}, []byte(payload))
}

// stateMachine is the brain of the protocol
func (sess *Session) stateMachine(frame *Frame) error {
	switch frame.command {
	case CmdConnect, CmdStomp:
		if err := sess.handleConnect(frame); err != nil {
			return err
		}

	case CmdSend:
		// If the message is part of an ongoing transaction
		if txID := frame.getHeader(HdrKeyTransaction); txID != "" {
			if err := bufferTxMessage(txID, frame); err != nil {
				return err
			}
			return nil
		}
		// Not part of transaction
		if err := publish(frame, ""); err != nil {
			return err
		}

	case CmdSubscribe:
		ack := HdrValAckAuto
		if _, ok := frame.headers[HdrKeyAck]; ok {
			ack = AckMode(frame.headers[HdrKeyAck])
		}
		if err := addSubscription(frame.headers[HdrKeyDestination], frame.headers[HdrKeyID], ack, sess); err != nil {
			return err
		}

	case CmdUnsubscribe:
		if err := removeSubscription(frame.headers[HdrKeyID]); err != nil {
			return err
		}

	case CmdAck:
		// if err := processAck(frame.headers[HdrKeyID]); err != nil {
		// 	return err
		// }

	case CmdNack:
		// if err := processNack(frame.headers[HdrKeyID]); err != nil {
		// 	return err
		// }

	case CmdBegin:
		if err := startTx(frame.headers[HdrKeyTransaction]); err != nil {
			return err
		}

	case CmdCommit:
		txID := frame.headers[HdrKeyTransaction]
		// Pick each message from TX buffer
		if err := foreachTx(txID, func(frameTx *Frame) error {
			// Send the message to each subscriber
			if err := publish(frameTx, txID); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return err
		}

	case CmdAbort:
		if err := dropTx(frame.headers[HdrKeyTransaction]); err != nil {
			return err
		}

	case CmdDisconnect:
		_ = cleanupSubscriptions(sess.sessionID)
		_ = sess.send(CmdReceipt, map[Header]string{HdrKeyReceiptID: frame.headers[HdrKeyReceipt]}, nil)
		_ = sess.conn.Close()
	}
	return nil
}

func (sess *Session) sendMessage(dest, subsID string, ackNum uint32, txID string, headers map[Header]string,
	body []byte,
) error {
	h := map[Header]string{
		HdrKeyDestination:  dest,
		HdrKeyMessageID:    uuid.NewString(),
		HdrKeySubscription: subsID,
	}
	h[HdrKeyAck] = fmtAckNum(dest, subsID, ackNum)
	if txID != "" {
		h[HdrKeyTransaction] = txID
	}
	for k, v := range headers {
		h[Header(strings.ToLower(string(k)))] = v
	}
	return sess.send(CmdMessage, h, body)
}

func (sess *Session) sendRaw(body []byte) error {
	sendIt := func() error {
		if _, err := sess.conn.Write(body); err != nil {
			log.Println(err)
			return err
		}
		return nil
	}

	if err := backoff.Retry(sendIt, backoff.NewExponentialBackOff()); err != nil {
		return err
	}

	return nil
}

func (sess *Session) send(cmd Command, headers map[Header]string, body []byte) error {
	f := NewFrame(cmd, headers, body)

	// Make this check optional later
	if err := f.Validate(ServerFrame); err != nil {
		return err
	}

	sendIt := func() error {
		if _, err := sess.conn.Write(f.Serialize()); err != nil {
			log.Println(err)
			return err
		}
		return nil
	}

	// Retry sending on error
	if err := backoff.Retry(sendIt, backoff.NewExponentialBackOff()); err != nil {
		return err
	}

	return nil
}

// handleConnect responds to the CONNECT message from client
func (sess *Session) handleConnect(f *Frame) error {
	// Authentication
	if sess.loginFunc != nil {
		login, passcode := f.getHeader(HdrKeyLogin), f.getHeader(HdrKeyPassCode)
		if err := sess.loginFunc(login, passcode); err != nil {
			_ = sess.sendError(errors.New("login failed"), "Authentication failed:\n"+err.Error())
			return errorMsg(errBrokerStateMachine, "Login error: "+err.Error())
		}
	}

	// Version negotiation
	ver := ""
	for _, v := range strings.Split(f.headers[HdrKeyAcceptVersion], ",") {
		if v == "1.2" {
			ver = "1.2"
			break
		}
	}
	if ver == "" {
		// Send version ERROR
		return errorMsg(errBrokerStateMachine, "Invalid client version received: "+f.getHeader(HdrKeyVersion))
	}

	// Heartbeat negotiation
	if hbVal := f.getHeader(HdrKeyHeartBeat); hbVal != "" {
		if err := sess.negotiateHeartbeats(hbVal); err != nil {
			return errorMsg(errBrokerStateMachine, "Heartbeat negotiation: "+err.Error())
		}
	}

	// Respond with CONNECTED
	if err := sess.send(CmdConnected, map[Header]string{
		HdrKeyVersion:   ver,
		HdrKeySession:   sess.sessionID,
		HdrKeyServer:    "go-proto-stomp/" + releaseVersion,
		HdrKeyHeartBeat: fmt.Sprintf("%d,%d", sess.hbSendIntervalMsec, sess.hbRecvIntervalMsec),
	}, nil); err != nil {
		return err
	}

	return nil
}

func (sess *Session) negotiateHeartbeats(hbVal string) error {
	intervals := strings.Split(hbVal, ",")
	if len(intervals) != 2 {
		return errorMsg(errBrokerStateMachine, "Invalid heartbeat header: "+hbVal)
	}

	// Send-HB negotiation
	clientSendInterval, err := strconv.Atoi(intervals[0])
	if err != nil {
		return errorMsg(errBrokerStateMachine,
			"Invalid heartbeat header send interval from client: "+hbVal)
	}
	if clientSendInterval == 0 || sess.hbRecvIntervalMsec == 0 {
		sess.hbRecvIntervalMsec = 0
	} else if clientSendInterval > sess.hbRecvIntervalMsec {
		sess.hbRecvIntervalMsec = clientSendInterval
	}

	// Receive-HB negotiation
	clientRecvInterval, err := strconv.Atoi(intervals[1])
	if err != nil {
		return errorMsg(errBrokerStateMachine,
			"Invalid heartbeat header receive interval from client: "+hbVal)
	}
	if clientRecvInterval == 0 || sess.hbSendIntervalMsec == 0 {
		sess.hbSendIntervalMsec = 0
	} else if clientRecvInterval > sess.hbSendIntervalMsec {
		sess.hbSendIntervalMsec = clientRecvInterval
	}

	// Schedule sending heartbeats by hbSendIntervalMsec
	if sess.hbSendIntervalMsec == 0 { // no heartbeats to be sent
		return nil
	}
	sess.hbJob, err = sched.Every(sess.hbSendIntervalMsec).Milliseconds().Tag(sess.sessionID).Do(
		func() {
			_ = sess.sendRaw([]byte("\n"))
		})
	if err != nil {
		return errorMsg(errBrokerStateMachine, "Heartbeat setup error: "+err.Error())
	}
	sched.StartAsync()

	return nil
}

// Broker lists the methods supported by the STOMP brokers
type Broker interface {
	// ListenAndServe is a blocking method that keeps accepting the client connections and handles the STOMP messages.
	ListenAndServe()

	// Shutdown should be called to bring down the underlying server gracefully.
	Shutdown()
}

// BrokerOpts is passed as an argument to StartBroker
type BrokerOpts struct {
	// Transport refers to the underlying protocol for STOMP.
	// Choices: TransportTCP, TransportWebsocket. Default: TransportTCP
	Transport Transport

	// Host is the name of the host or IP to bind the server to. Default: localhost
	Host string

	// Port is the port number for the server to listen on. Default: 61613 (DefaultPort)
	Port string

	// LoginFunc is a user defined function for authenticating the user. Default: nil
	// It is of the form `func(login, passcode string) error`
	LoginFunc LoginFunc

	// HeartbeatSendIntervalMsec is the interval in milliseconds by which the broker can send heartbeats.
	// The broker will negotiate using this value with the client. Default: 0 (no heartbeats)
	// It will not send the heartbeats by an interval any smaller than this value.
	HeartbeatSendIntervalMsec int

	// HeartbeatReceiveIntervalMsec is the interval in milliseconds by which the broker can receive heartbeats.
	// The broker will negotiate using this value with the client. Default: 0 (no heartbeats)
	// This is to tell the client that the broker cannot receive heartbeats by any shorter interval than this value.
	HeartbeatReceiveIntervalMsec int
}

// StartBroker is the entry point for the STOMP broker.
func StartBroker(opts *BrokerOpts) (Broker, error) {
	var broker Broker
	var err error

	// Set default values
	if opts.Host == "" {
		opts.Host = "localhost"
	}
	if opts.Port == "" {
		opts.Port = DefaultPort
	}
	if opts.Transport == "" {
		opts.Transport = TransportTCP
	}
	if opts.HeartbeatSendIntervalMsec < 0 {
		opts.HeartbeatSendIntervalMsec = 0
	}
	if opts.HeartbeatReceiveIntervalMsec < 0 {
		opts.HeartbeatReceiveIntervalMsec = 0
	}

	switch opts.Transport {
	case TransportTCP:
		var tcp *tcpBroker
		if tcp, err = startTcpBroker(opts); err != nil {
			return nil, err
		}
		broker = tcp
	case TransportWebsocket:
		var wss *wssBroker
		if wss, err = startWebsocketBroker(opts); err != nil {
			return nil, err
		}
		broker = wss
	}
	return broker, nil
}
