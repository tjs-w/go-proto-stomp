package stomp

import (
	"fmt"
	"strings"

	set "github.com/deckarep/golang-set"
)

// Command represents the STOMP message-type
type Command string

// Server Frame Commands
const (
	CmdConnected Command = "CONNECTED"
	CmdMessage   Command = "MESSAGE"
	CmdReceipt   Command = "RECEIPT"
	CmdError     Command = "ERROR"
)

var serverCmdSet = set.NewSet(CmdConnected, CmdMessage, CmdReceipt, CmdError)

// Client Frame Commands
const (
	CmdConnect     Command = "CONNECT"
	CmdStomp       Command = "STOMP"
	CmdSend        Command = "SEND"
	CmdSubscribe   Command = "SUBSCRIBE"
	CmdUnsubscribe Command = "UNSUBSCRIBE"
	CmdAck         Command = "ACK"
	CmdNack        Command = "NACK"
	CmdBegin       Command = "BEGIN"
	CmdCommit      Command = "COMMIT"
	CmdAbort       Command = "ABORT"
	CmdDisconnect  Command = "DISCONNECT"
)

var clientCmdSet = set.NewSet(
	CmdConnect, CmdStomp, CmdSend, CmdSubscribe,
	CmdUnsubscribe, CmdAck, CmdNack, CmdBegin,
	CmdCommit, CmdAbort, CmdDisconnect,
)

// Header represents the header-keys in the STOMP Frame
type Header string

// Frame headers
const (
	HdrKeyAcceptVersion Header = "accept-version"
	HdrKeyAck           Header = "ack"
	HdrKeyContentLength Header = "content-length"
	HdrKeyContentType   Header = "content-type"
	HdrKeyDestination   Header = "destination"
	HdrKeyHeartBeat     Header = "heart-beat"
	HdrKeyHost          Header = "host"
	HdrKeyID            Header = "id"
	HdrKeyLogin         Header = "login"
	HdrKeyMessage       Header = "message"
	HdrKeyMessageID     Header = "message-id"
	HdrKeyPassCode      Header = "passcode"
	HdrKeyReceipt       Header = "receipt"
	HdrKeyReceiptID     Header = "receipt-id"
	HdrKeyServer        Header = "server"
	HdrKeySession       Header = "session"
	HdrKeySubscription  Header = "subscription"
	HdrKeyTransaction   Header = "transaction"
	HdrKeyVersion       Header = "version"
)

type AckMode string

// Header values for AckMode
const (
	HdrValAckAuto             AckMode = "auto"
	HdrValAckClient           AckMode = "client"
	HdrValAckClientIndividual AckMode = "client-individual"
)

type typeHeaders struct {
	required set.Set
	optional set.Set
}

// validationMap maps the Command to the required and optional Header fields
var validationMap = map[Command]typeHeaders{
	CmdConnect: {
		required: set.NewSet(
			HdrKeyHost,
			HdrKeyAcceptVersion,
		),
		optional: set.NewSet(
			HdrKeyLogin,
			HdrKeyPassCode,
			HdrKeyHeartBeat,
		),
	},

	CmdStomp: {
		required: set.NewSet(
			HdrKeyHost,
			HdrKeyAcceptVersion,
		),
		optional: set.NewSet(
			HdrKeyLogin,
			HdrKeyPassCode,
			HdrKeyHeartBeat,
		),
	},

	CmdConnected: {
		required: set.NewSet(
			HdrKeyVersion,
		),
		optional: set.NewSet(
			HdrKeySession,
			HdrKeyServer,
			HdrKeyHeartBeat,
		),
	},

	CmdSend: {
		required: set.NewSet(
			HdrKeyDestination,
		),
		optional: set.NewSet(
			HdrKeyTransaction,
		),
	},

	CmdSubscribe: {
		required: set.NewSet(
			HdrKeyDestination,
			HdrKeyID,
		),
		optional: set.NewSet(
			HdrKeyAck,
		),
	},

	CmdUnsubscribe: {
		required: set.NewSet(
			HdrKeyID,
		),
		optional: set.NewSet(),
	},

	CmdAck: {
		required: set.NewSet(
			HdrKeyID,
		),
		optional: set.NewSet(
			HdrKeyTransaction,
		),
	},

	CmdNack: {
		required: set.NewSet(
			HdrKeyID,
		),
		optional: set.NewSet(
			HdrKeyTransaction,
		),
	},

	CmdBegin: {
		required: set.NewSet(
			HdrKeyTransaction,
		),
		optional: set.NewSet(),
	},

	CmdCommit: {
		required: set.NewSet(
			HdrKeyTransaction,
		),
		optional: set.NewSet(),
	},

	CmdAbort: {
		required: set.NewSet(
			HdrKeyTransaction,
		),
		optional: set.NewSet(),
	},

	CmdDisconnect: {
		required: set.NewSet(),
		optional: set.NewSet(
			HdrKeyReceipt,
		),
	},

	CmdMessage: {
		required: set.NewSet(
			HdrKeyDestination,
			HdrKeyMessageID,
			HdrKeySubscription,
		),
		optional: set.NewSet(
			HdrKeyAck,
		),
	},

	CmdReceipt: {
		required: set.NewSet(
			HdrKeyReceiptID,
		),
		optional: set.NewSet(),
	},

	CmdError: {
		required: set.NewSet(),
		optional: set.NewSet(
			HdrKeyMessage,
		),
	},
}

// FrameSource is the source of frame: either Broker or Client
type FrameSource int

// Modes: server/client
const (
	ServerFrame FrameSource = 0
	ClientFrame FrameSource = 1
)

// Frame represents the stomp protocol frame
type Frame struct {
	command Command
	headers map[Header]string
	body    []byte
}

// NewFrame creates an empty new Frame
func NewFrame(cmd Command, headers map[Header]string, body []byte) *Frame {
	return &Frame{
		command: cmd,
		headers: headers,
		body:    body,
	}
}

// NewFrameFromBytes creates a new Frame that is populated with deserialized raw input
func NewFrameFromBytes(raw []byte) (*Frame, error) {
	f := &Frame{}
	if err := f.Deserialize(raw); err != nil {
		return nil, err
	}
	return f, nil
}

func (f *Frame) customHeadersAllowed() bool {
	return f.command == CmdMessage || f.command == CmdError || f.command == CmdSend
}

// checkValidEscapes returns false if it finds any escape sequences other than these: '\\', '\n', '\r', '\c'
func checkValidEscapes(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			if i+1 == len(s) {
				return false
			}
			if s[i+1] != 'n' && s[i+1] != 'r' && s[i+1] != 'c' && s[i+1] != '\\' {
				return false
			}
			i++
		}
	}
	return true
}

func (f *Frame) validateHeaders() error {
	// Are the headers in frame among REQUIRED or OPTIONAL headers for the CMD?
	// Exception is for MESSAGE, ERROR and SEND frames that can have CUSTOM headers.
	req := validationMap[f.command].required
	opt := validationMap[f.command].optional
	for h, v := range f.headers {
		if !req.Contains(h) && !opt.Contains(h) && !f.customHeadersAllowed() {
			return errorMsg(errProtocolFrame, fmt.Sprintf("Invalid header '%s' for command '%s'", h, f.command))
		}

		// Headers and their values must not contain any escape char other than: '\\', '\n', '\r', '\c'
		if !checkValidEscapes(string(h)) || !checkValidEscapes(v) {
			return errorMsg(errProtocolFrame,
				fmt.Sprintf("Invalid escape sequence in header '%s:%s' for command '%s'", h, v, f.command))
		}
	}

	// Are all the required headers for the given type present?
	for h := range req.Iter() {
		if _, ok := f.headers[h.(Header)]; !ok {
			return errorMsg(errProtocolFrame,
				fmt.Sprintf("Missing required header '%s' for command '%s'", h, f.command))
		}
	}

	return nil
}

func (f *Frame) validateSeverCommand() error {
	if !serverCmdSet.Contains(f.command) {
		return errorMsg(errProtocolFrame, fmt.Sprintf("'%s' is not a valid Broker/Server command", f.command))
	}
	return nil
}

func (f *Frame) validateClientCommand() error {
	if !clientCmdSet.Contains(f.command) {
		return errorMsg(errProtocolFrame, fmt.Sprintf("'%s' is not a valid Client command", f.command))
	}
	return nil
}

// Validate checks both client and server STOMP frames. It also checks if the mandatory headers are present for a
// given message.
func (f *Frame) Validate(s FrameSource) error {
	if s == ServerFrame {
		if err := f.validateSeverCommand(); err != nil {
			return err
		}
	} else if s == ClientFrame {
		if err := f.validateClientCommand(); err != nil {
			return err
		}
	} else {
		return errorMsg(errInvalidArg,
			fmt.Sprintf("FrameSource must be either ServerMode or ClientMode, received: '%d'", s))
	}

	if err := f.validateHeaders(); err != nil {
		return err
	}

	return nil
}

// Serialize marshals the Frame struct into wire-format of the protocol
func (f *Frame) Serialize() []byte {
	return serialize(f)
}

// Deserialize takes the wire-format and unmarshalls it into Frame struct. It also internally validates the
// wire-format while parsing to some extent.
func (f *Frame) Deserialize(buf []byte) error {
	return deserialize(buf, f)
}

// String method gives a printable version of the Frame
func (f Frame) String() string {
	sb := strings.Builder{}
	sb.WriteString("\n===\n")

	sb.WriteString(string(f.command) + "\n")
	// Sort headers
	for _, k := range f.getSortedHeaderKeys() {
		sb.WriteString(escape(k) + ":" + escape(f.headers[Header(k)]) + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprint(string(f.body)) + "<NUL>")

	sb.WriteString("\n===\n")
	return sb.String()
}

func (f *Frame) getHeader(h Header) string {
	if v, ok := f.headers[h]; ok {
		return v
	}
	return ""
}
