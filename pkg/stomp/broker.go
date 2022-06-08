package stomp

import (
	_ "embed"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/cenkalti/backoff"
	"github.com/google/uuid"
)

//go:generate sh -c "git describe --tags --abbrev=0 | tee version.txt"
var (
	//go:embed version*.txt
	releaseVersion string
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

// Session handles the STOMP client session on connection
type Session struct {
	conn       net.Conn
	sessionID  string
	loginFunc  LoginFunc
	wgSessions *sync.WaitGroup
}

// NewSession creates a new session object & maintains the session state internally
func NewSession(conn net.Conn, loginFunc LoginFunc, wg *sync.WaitGroup) *Session {
	return &Session{
		conn:       conn,
		loginFunc:  loginFunc,
		sessionID:  uuid.NewString(),
		wgSessions: wg,
	}
}

// LoginFunc represents the user-defined authentication function
type LoginFunc func(login, passcode string) error

// Start begins the STOMP session with the Client
func (sess *Session) Start() {
	defer sess.cleanup()
	sess.wgSessions.Add(1)
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

	if err := backoff.Retry(sendIt, backoff.NewExponentialBackOff()); err != nil {
		return err
	}

	return nil
}

// handleConnect responds to the CONNECT message from client
func (sess *Session) handleConnect(f *Frame) error {
	// Version
	ver := ""
	for _, v := range strings.Split(f.headers[HdrKeyAcceptVersion], ",") {
		if v == "1.2" {
			ver = "1.2"
			break
		}
	}
	if ver == "" {
		// Send version ERROR
		err := sess.sendError(errors.New("version mismatch"),
			"Server supported versions: 1.2. Received:"+f.String())
		return errorMsg(errBrokerStateMachine, "Invalid client version received: "+f.String()+"::"+err.Error())
	}

	// Authentication
	if sess.loginFunc != nil {
		login, passcode := f.getHeader(HdrKeyLogin), f.getHeader(HdrKeyPassCode)
		if err := sess.loginFunc(login, passcode); err != nil {
			_ = sess.sendError(errors.New("login failed"), "Authentication failed:\n"+err.Error())
			return errorMsg(errBrokerStateMachine, "Login error: "+err.Error())
		}
	}

	// ToDo Heartbeats

	// Respond with CONNECTED
	if err := sess.send(CmdConnected, map[Header]string{
		HdrKeyVersion:   ver,
		HdrKeySession:   sess.sessionID,
		HdrKeyServer:    "go-proto-stomp/" + releaseVersion,
		HdrKeyHeartBeat: "0,0",
	}, nil); err != nil {
		return err
	}

	return nil
}

// Broker lists the methods supported by the STOMP brokers
type Broker interface {
	// ListenAndServe is a blocking method that keeps accepting the client connections and handles the STOMP messages.
	ListenAndServe()

	// Shutdown should be called to bring down the underlying server gracefully.
	Shutdown()
}

// StartBroker is the entry point for the STOMP broker.
// It accepts `transport` which could be either TCP or Websocket; the host and the port the server must start on; and
// a loginFunc [func(login, passcode string) error] which is a user defined function for authenticating the user.
func StartBroker(transport Transport, host, port string, loginFunc LoginFunc) (Broker, error) {
	var broker Broker
	var err error
	switch transport {
	case TransportTCP:
		var tcp *tcpBroker
		if tcp, err = startTcpBroker(host, port, loginFunc); err != nil {
			return nil, err
		}
		broker = tcp
	case TransportWebsocket:
		var wss *wssBroker
		if wss, err = startWebsocketBroker(host, port, loginFunc); err != nil {
			return nil, err
		}
		broker = wss
	}
	return broker, nil
}

// ReleaseVersion returns the version of the go-proto-stomp module
func ReleaseVersion() string {
	return releaseVersion
}
