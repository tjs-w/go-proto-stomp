package stomp

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/cenkalti/backoff"
	"github.com/go-co-op/gocron"
	"github.com/google/uuid"
)

const (
	// disconnectID is used as a `receipt` header value in the DISCONNECT message from client
	disconnectID = "BYE-BYE!"
)

// UserMessage represents the messages and the user-headers to be received by the user
type UserMessage struct {
	Headers map[string]string // STOMP and custom headers received in MESSAGE
	Body    []byte            // MESSAGE payload
}

// Subscription represents the state of subscription
type Subscription struct {
	c           *ClientHandler
	SubsID      string
	Destination string
}

// Transaction represents the state of transaction
type Transaction struct {
	c    *ClientHandler
	TxID string
}

// MessageHandlerFunc is the function-type for user-defined function to handle the messages
type MessageHandlerFunc func(message *UserMessage)

// ClientHandler is the control struct for Client's connection with the STOMP Broker
type ClientHandler struct {
	SessionID      string             // Session ID for the connection with the STOMP Broker
	conn           net.Conn           // Connection to the server/broker
	host           string             // Virtual-host on the STOMP broker
	login          string             // Username for the login to STOMP broker
	passcode       string             // Password to log in to the STOMP broker
	hbSendInterval int                // Send-interval in milliseconds from client
	hbRecvInterval int                // Receive-interval in milliseconds on client
	hbJob          *gocron.Job        // Heartbeat sending job
	msgHandler     MessageHandlerFunc // Callback to process the MESSAGE
}

// ClientOpts provides the options as argument to NewClientHandler
type ClientOpts struct {
	VirtualHost              string             // Virtual host
	Login                    string             // AuthN Username
	Passcode                 string             // AuthN Password
	HeartbeatSendInterval    int                // Sending interval of heartbeats in milliseconds
	HeartbeatReceiveInterval int                // Receiving interval of heartbeats in milliseconds
	MessageHandler           MessageHandlerFunc // User-defined callback function to handle MESSAGE
}

// NewClientHandler creates the Client for STOMP
func NewClientHandler(transport Transport, host, port string, opts *ClientOpts) *ClientHandler {
	var conn net.Conn
	var err error

	switch transport {
	case TransportTCP:
		conn, err = startTcpClient(host, port)
	case TransportWebsocket:
		conn, err = startWebsocketClient(host, port)
	}
	if err != nil {
		log.Fatal(err)
	}

	if opts == nil {
		opts = &ClientOpts{}
	}

	if opts.VirtualHost == "" {
		opts.VirtualHost = conn.RemoteAddr().String()
	}
	return &ClientHandler{
		conn:           conn,
		host:           opts.VirtualHost,
		login:          opts.Login,
		passcode:       opts.Passcode,
		hbSendInterval: opts.HeartbeatSendInterval,
		hbRecvInterval: opts.HeartbeatReceiveInterval,
		msgHandler:     opts.MessageHandler,
	}
}

// SetMessageHandler accepts the user-defined function to handle the messages
func (c *ClientHandler) SetMessageHandler(handlerFunc MessageHandlerFunc) {
	c.msgHandler = handlerFunc
}

// Connect connects with the broker and starts listening to the messages from broker
func (c *ClientHandler) Connect(useStompCmd bool) error {
	if err := c.connect(useStompCmd); err != nil {
		return err
	}

	go func() {
		for raw := range frameScanner(c.conn) {
			frame, err := NewFrameFromBytes(raw)
			if err != nil {
				log.Println(err)
				break
			}
			if err = frame.Validate(ServerFrame); err != nil {
				log.Println(err)
				break
			}
			if err = c.stateMachine(frame); err != nil {
				log.Println(err)
				break
			}
		}

		// Cleanup
		if c.hbJob != nil {
			sched.RemoveByReference(c.hbJob)
		}
	}()
	return nil
}

// stateMachine is the brain of STOMP client
func (c *ClientHandler) stateMachine(frame *Frame) error {
	switch frame.command {
	case CmdConnected:
		c.SessionID = frame.headers[HdrKeySession]
		if hbVal := frame.getHeader(HdrKeyHeartBeat); hbVal != "" {
			if err := c.negotiateHeartbeats(hbVal); err != nil {
				return err
			}
		}
	case CmdMessage:
		if c.msgHandler != nil {
			c.msgHandler(c.getUserMessage(frame))
		}
	case CmdReceipt:
		if frame.headers[HdrKeyReceiptID] == disconnectID {
			_ = c.conn.Close()
			return errors.New("bye") // Returning error will close the connection
		}
	case CmdError:
		if err := c.Disconnect(); err != nil {
			return err
		}
	}
	return nil
}

func (c *ClientHandler) send(cmd Command, headers map[Header]string, body []byte) error {
	f := NewFrame(cmd, headers, body)
	if err := f.Validate(ClientFrame); err != nil {
		return err
	}

	sendIt := func() error {
		if _, err := c.conn.Write(f.Serialize()); err != nil {
			return err
		}
		return nil
	}

	if err := backoff.Retry(sendIt, backoff.NewExponentialBackOff()); err != nil {
		return err
	}

	return nil
}

func (c *ClientHandler) sendRaw(body []byte) error {
	sendIt := func() error {
		if _, err := c.conn.Write(body); err != nil {
			return err
		}
		return nil
	}

	if err := backoff.Retry(sendIt, backoff.NewExponentialBackOff()); err != nil {
		return err
	}

	return nil
}

func (c *ClientHandler) negotiateHeartbeats(hbVal string) error {
	intervals := strings.Split(hbVal, ",")
	if len(intervals) != 2 {
		return errorMsg(errClientStateMachine, "Invalid heartbeat header: "+hbVal)
	}

	// Send-HB negotiation
	brokerSendInterval, err := strconv.Atoi(intervals[0])
	if err != nil {
		return errorMsg(errClientStateMachine,
			"Invalid heartbeat header send interval from client: "+hbVal)
	}
	if brokerSendInterval == 0 || c.hbRecvInterval == 0 {
		c.hbRecvInterval = 0
	} else if brokerSendInterval > c.hbRecvInterval {
		c.hbRecvInterval = brokerSendInterval
	}

	// Receive-HB negotiation
	brokerRecvInterval, err := strconv.Atoi(intervals[1])
	if err != nil {
		return errorMsg(errClientStateMachine,
			"Invalid heartbeat header receive interval from client: "+hbVal)
	}
	if brokerRecvInterval == 0 || c.hbSendInterval == 0 {
		c.hbSendInterval = 0
	} else if brokerRecvInterval > c.hbSendInterval {
		c.hbSendInterval = brokerRecvInterval
	}

	// Schedule sending heartbeats by hbSendInterval
	if c.hbSendInterval == 0 { // no heartbeats to be sent
		return nil
	}
	c.hbJob, err = sched.Every(c.hbSendInterval).Milliseconds().Tag(c.SessionID).Do(
		func() {
			_ = c.sendRaw([]byte("\n"))
		})
	if err != nil {
		return errorMsg(errClientStateMachine, "Heartbeat setup error: "+err.Error())
	}
	sched.StartAsync()

	return nil
}

func (c *ClientHandler) getUserMessage(f *Frame) *UserMessage {
	userHeaders := map[string]string{}
	for h, v := range f.headers {
		userHeaders[string(h)] = v
	}
	return &UserMessage{
		Headers: userHeaders,
		Body:    f.body,
	}
}

func (c *ClientHandler) connect(useStomp bool) error {
	headers := map[Header]string{
		HdrKeyAcceptVersion: "1.2",
		HdrKeyHost:          c.host,
	}
	if c.login != "" {
		headers[HdrKeyLogin] = c.login
		headers[HdrKeyPassCode] = c.passcode
	}
	if c.hbSendInterval != 0 || c.hbRecvInterval != 0 {
		headers[HdrKeyHeartBeat] = fmt.Sprintf("%d,%d", c.hbSendInterval, c.hbRecvInterval)
	}

	cmd := CmdConnect
	if useStomp {
		cmd = CmdStomp
	}
	return c.send(cmd, headers, nil)
}

func (c *ClientHandler) Send(dest string, body []byte, contentType string, customHeaders map[string]string) error {
	h := map[Header]string{
		HdrKeyDestination:   dest,
		HdrKeyContentType:   contentType,
		HdrKeyContentLength: strconv.Itoa(len(body)),
	}
	for k, v := range customHeaders {
		h[Header(k)] = v
	}
	return c.send(CmdSend, h, body)
}

func (c *ClientHandler) Disconnect() error {
	return c.send(CmdDisconnect, map[Header]string{HdrKeyReceipt: disconnectID}, nil)
}

func (c *ClientHandler) Subscribe(dest string, mode AckMode) (*Subscription, error) {
	subID := uuid.NewString()
	if mode == "" {
		mode = HdrValAckAuto
	}
	h := map[Header]string{
		HdrKeyID:          subID,
		HdrKeyDestination: dest,
		HdrKeyAck:         string(mode),
	}
	if err := c.send(CmdSubscribe, h, nil); err != nil {
		return nil, err
	}
	return &Subscription{c: c, SubsID: subID, Destination: dest}, nil
}

func (s *Subscription) Unsubscribe() error {
	return s.c.send(CmdUnsubscribe, map[Header]string{HdrKeyID: s.SubsID}, nil)
}

func (c *ClientHandler) BeginTransaction() (*Transaction, error) {
	txID := uuid.NewString()
	if err := c.send(CmdBegin, map[Header]string{HdrKeyTransaction: txID}, nil); err != nil {
		return nil, err
	}
	return &Transaction{c: c, TxID: txID}, nil
}

func (t *Transaction) Send(dest string, body []byte, contentType string, headers map[string]string) error {
	if t.c == nil {
		return errorMsg(errProtocolFrame, "Send on closed transaction")
	}
	hdr := map[string]string{}
	for k, v := range headers {
		hdr[strings.ToLower(k)] = v
	}
	hdr[string(HdrKeyTransaction)] = t.TxID
	return t.c.Send(dest, body, contentType, hdr)
}

func (t *Transaction) AbortTransaction() error {
	if t.c == nil {
		return errorMsg(errProtocolFrame, "Abort on closed transaction")
	}
	if err := t.c.send(CmdAbort, map[Header]string{HdrKeyTransaction: t.TxID}, nil); err != nil {
		return err
	}
	t.c = nil
	return nil
}

func (t *Transaction) CommitTransaction() error {
	if t.c == nil {
		return errorMsg(errProtocolFrame, "Commit on closed transaction")
	}
	if err := t.c.send(CmdCommit, map[Header]string{HdrKeyTransaction: t.TxID}, nil); err != nil {
		return err
	}
	t.c = nil
	return nil
}

// func (c *ClientHandler) ack(id string, txID string) error {
// 	m := map[Header]string{HdrKeyID: id}
// 	if txID != "" {
// 		m[HdrKeyTransaction] = txID
// 	}
// 	return c.send(CmdAck, m, nil)
// }
//
// func (c *ClientHandler) nack(id string, txID string) error {
// 	m := map[Header]string{HdrKeyID: id}
// 	if txID != "" {
// 		m[HdrKeyTransaction] = txID
// 	}
// 	return c.send(CmdNack, m, nil)
// }
