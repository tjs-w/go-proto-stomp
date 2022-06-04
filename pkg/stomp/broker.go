package stomp

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// SessionHandler handles the STOMP client session on connection
type SessionHandler struct {
	conn       net.Conn
	sessionID  string
	loginFunc  LoginFunc
	wgSessions *sync.WaitGroup
}

// NewSessionHandler creates a new session object & maintains the session state internally
func NewSessionHandler(conn net.Conn, loginFunc LoginFunc, wg *sync.WaitGroup) *SessionHandler {
	return &SessionHandler{
		conn:       conn,
		loginFunc:  loginFunc,
		sessionID:  uuid.NewString(),
		wgSessions: wg,
	}
}

// LoginFunc represents the user-defined authentication function
type LoginFunc func(login, passcode string) error

// Start begins the STOMP session with the Client
func (sh *SessionHandler) Start() {
	defer sh.cleanup()
	sh.wgSessions.Add(1)
	for raw := range FrameScanner(sh.conn) {
		frame, err := NewFrameFromBytes(raw)
		if err != nil {
			_ = sh.sendError(err, fmt.Sprint("Frame serialization error:"+frame.String()))
			return
		}
		if err = frame.Validate(ClientFrame); err != nil {
			_ = sh.sendError(err, fmt.Sprint("Frame validation error:"+frame.String()))
			return
		}

		if err = sh.stateMachine(frame); err != nil {
			log.Println(err)
			return
		}
	}
}

func (sh *SessionHandler) cleanup() {
	sh.conn.Close()
	sh.wgSessions.Done()
}

// sendError is the helper function to send the ERROR frames
func (sh *SessionHandler) sendError(err error, payload string) error {
	return sh.send(CmdError, map[Header]string{
		HdrKeyContentType:   "text/plain",
		HdrKeyContentLength: strconv.Itoa(len(payload)),
		HdrKeyMessage:       err.Error(),
	}, []byte(payload))
}

// stateMachine is the brain of the protocol
func (sh *SessionHandler) stateMachine(frame *Frame) error {
	switch frame.command {
	case CmdConnect, CmdStomp:
		if err := sh.handleConnect(frame); err != nil {
			return err
		}

	case CmdSend:
		// If the message is part of an ongoing transaction
		if txID, ok := frame.headers[HdrKeyTransaction]; ok && frame.headers[HdrKeyTransaction] != "" {
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
		var ack = HdrValAckAuto
		if _, ok := frame.headers[HdrKeyAck]; ok {
			ack = AckMode(frame.headers[HdrKeyAck])
		}
		if err := addSubscription(frame.headers[HdrKeyDestination], frame.headers[HdrKeyID], ack, sh); err != nil {
			return err
		}

	case CmdUnsubscribe:
		if err := removeSubscription(frame.headers[HdrKeyID]); err != nil {
			return err
		}

	case CmdAck:
		if err := processAck(frame.headers[HdrKeyID]); err != nil {
			return err
		}

	case CmdNack:
		if err := processNack(frame.headers[HdrKeyID]); err != nil {
			return err
		}

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
		_ = cleanupSubscriptions(sh.sessionID)
		_ = sh.send(CmdReceipt, map[Header]string{HdrKeyReceiptID: frame.headers[HdrKeyReceipt]}, nil)
		sh.conn.Close()
	}
	return nil
}

func (sh *SessionHandler) sendMessage(dest, subsID string, ackNum uint32, txID string, headers map[Header]string,
	body []byte) error {
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
	return sh.send(CmdMessage, h, body)
}

func (sh *SessionHandler) send(cmd Command, headers map[Header]string, body []byte) error {
	f := NewFrame(cmd, headers, body)

	// Make this check optional later
	if err := f.Validate(ServerFrame); err != nil {
		return err
	}

	if _, err := sh.conn.Write(f.Serialize()); err != nil {
		fmt.Println(err)
	}
	return nil
}

// handleConnect responds to the CONNECT message from client
func (sh *SessionHandler) handleConnect(f *Frame) error {
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
		err := sh.sendError(errors.New("version mismatch"),
			"Server supported versions: 1.2. Received:"+f.String())
		return errorMsg(errBrokerStateMachine, "Invalid client version received: "+f.String()+"::"+err.Error())
	}

	// Authentication
	if sh.loginFunc != nil {
		login, passcode := f.getHeader(HdrKeyLogin), f.getHeader(HdrKeyPassCode)
		if err := sh.loginFunc(login, passcode); err != nil {
			_ = sh.sendError(errors.New("login failed"), "Authentication failed:\n"+err.Error())
			return errorMsg(errBrokerStateMachine, "Login error: "+err.Error())
		}
	}

	// ToDo Heartbeats

	// Respond with CONNECTED
	if err := sh.send(CmdConnected, map[Header]string{
		HdrKeyVersion:   ver,
		HdrKeySession:   sh.sessionID,
		HdrKeyServer:    "go-proto-stomp/v1.0.0",
		HdrKeyHeartBeat: "0,0",
	}, nil); err != nil {
		return err
	}

	return nil
}

type Broker interface {
	ListenAndServe()
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
