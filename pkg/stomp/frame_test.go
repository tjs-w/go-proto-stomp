package stomp

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var deserializedFrames = []struct {
	name  string
	typ   string
	frame Frame
}{
	{
		name: string(CmdConnect),
		typ:  "client",
		frame: Frame{
			command: CmdConnect,
			headers: map[Header]string{
				HdrKeyAcceptVersion: "1.0,1.1,2.0",
				HdrKeyHost:          "stomp.example.com",
				HdrKeyLogin:         "peter@parker.com",
				HdrKeyPassCode:      "maryjane",
				HdrKeyHeartBeat:     "0,0",
			},
		},
	},
	{
		name: string(CmdStomp),
		typ:  "client",
		frame: Frame{
			command: CmdStomp,
			headers: map[Header]string{
				HdrKeyAcceptVersion: "1.0,1.1,2.0",
				HdrKeyHost:          "stomp.example.com",
				HdrKeyLogin:         "peter@parker.com",
				HdrKeyPassCode:      "maryjane",
				HdrKeyHeartBeat:     "0,0",
			},
		},
	},
	{
		name: string(CmdConnected),
		typ:  "server",
		frame: Frame{
			command: CmdConnected,
			headers: map[Header]string{
				HdrKeyVersion:   "1.1",
				HdrKeyServer:    "Apache/1.3.9",
				HdrKeySession:   "78",
				HdrKeyHeartBeat: "0,0",
			},
		},
	},
	{
		name: string(CmdSend),
		typ:  "client",
		frame: Frame{
			command: CmdSend,
			headers: map[Header]string{
				HdrKeyDestination: "/queue/a",
				HdrKeyTransaction: "tx10",
				HdrKeyContentType: "text/plain",
			},
			body: []byte("hello queue a\n"),
		},
	},
	{
		name: string(CmdSubscribe),
		typ:  "client",
		frame: Frame{
			command: CmdSubscribe,
			headers: map[Header]string{
				HdrKeyID:          "0",
				HdrKeyDestination: "/queue/foo",
				HdrKeyAck:         string(HdrValAckClient),
			},
		},
	},
	{
		name: string(CmdUnsubscribe),
		typ:  "client",
		frame: Frame{
			command: CmdUnsubscribe,
			headers: map[Header]string{
				HdrKeyID: "0",
			},
		},
	},
	{
		name: string(CmdAck),
		typ:  "client",
		frame: Frame{
			command: CmdAck,
			headers: map[Header]string{
				HdrKeyID:          "12345",
				HdrKeyTransaction: "tx1",
			},
		},
	},
	{
		name: string(CmdNack),
		typ:  "client",
		frame: Frame{
			command: CmdNack,
			headers: map[Header]string{
				HdrKeyID:          "12345",
				HdrKeyTransaction: "tx1",
			},
		},
	},
	{
		name: string(CmdBegin),
		typ:  "client",
		frame: Frame{
			command: CmdBegin,
			headers: map[Header]string{
				HdrKeyTransaction: "tx1",
			},
		},
	},
	{
		name: string(CmdCommit),
		typ:  "client",
		frame: Frame{
			command: CmdCommit,
			headers: map[Header]string{
				HdrKeyTransaction: "tx1",
			},
		},
	},
	{
		name: string(CmdAbort),
		typ:  "client",
		frame: Frame{
			command: CmdAbort,
			headers: map[Header]string{
				HdrKeyTransaction: "tx1",
			},
		},
	},
	{
		name: string(CmdDisconnect),
		typ:  "client",
		frame: Frame{
			command: CmdDisconnect,
			headers: map[Header]string{
				HdrKeyReceipt: "77",
			},
		},
	},
	{
		name: string(CmdReceipt),
		typ:  "server",
		frame: Frame{
			command: CmdReceipt,
			headers: map[Header]string{
				HdrKeyReceiptID: "77",
			},
		},
	},
	{
		name: string(CmdMessage),
		typ:  "server",
		frame: Frame{
			command: CmdMessage,
			headers: map[Header]string{
				HdrKeySubscription: "0",
				HdrKeyMessageID:    "007",
				HdrKeyDestination:  "/queue/a",
				HdrKeyContentType:  "text/plain",
				HdrKeyAck:          "1",
			},
			body: []byte("hello queue a"),
		},
	},
	{
		name: string(CmdError),
		typ:  "server",
		frame: Frame{
			command: CmdError,
			headers: map[Header]string{
				HdrKeyVersion:     "1.2,2.1",
				HdrKeyContentType: "text/plain",
			},
			body: []byte("Supported protocol versions are 1.2 2.1"),
		},
	},
	{
		name: string(CmdError) + "_2",
		typ:  "server",
		frame: Frame{
			command: CmdError,
			headers: map[Header]string{
				HdrKeyReceiptID:     "message-12345",
				HdrKeyContentType:   "text/plain",
				HdrKeyContentLength: "170",
				HdrKeyMessage:       "malformed frame received",
			},
			body: []byte(`The message:
=====
MESSAGE
destined:/queue/a
receipt:message-12345

Hello queue a!
=====
Did not contain a destination header, which is REQUIRED
for message propagation.
`),
		},
	},
}

func getGoldenFrame(testName, typ string, t *testing.T) []byte {
	b, err := ioutil.ReadFile(fmt.Sprintf("testdata/frames/%s/%s.golden.txt", typ, testName))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestFrame_Deserialize(t *testing.T) {
	for _, expected := range deserializedFrames {
		t.Run(expected.name, func(t *testing.T) {
			serializedFrame := getGoldenFrame(expected.name, expected.typ, t)
			f := Frame{}
			if err := f.Deserialize(serializedFrame); err != nil {
				t.Error(err)
				return
			}
			if !cmp.Equal(f, expected.frame, cmp.AllowUnexported(Frame{})) {
				t.Error(cmp.Diff(f, expected.frame, cmp.AllowUnexported(Frame{})))
			}
		})
	}
}

func TestFrame_Serialize(t *testing.T) {
	for _, testFrame := range deserializedFrames {
		t.Run(testFrame.name, func(t *testing.T) {
			expectedFrame := getGoldenFrame(testFrame.name, testFrame.typ, t)
			expectedFrame = []byte(strings.TrimRight(string(expectedFrame), "\n"))
			actualFrame := testFrame.frame.Serialize()
			if !bytes.Equal(actualFrame, expectedFrame) {
				t.Error(cmp.Diff(actualFrame, expectedFrame), cmp.AllowUnexported(Frame{}))
			}
		})
	}
}

func TestFrame_Validate(t *testing.T) {
	for _, testFrame := range deserializedFrames {
		t.Run(testFrame.name, func(t *testing.T) {
			if serverCmdSet.Contains(testFrame.frame.command) {
				if err := testFrame.frame.Validate(ServerFrame); err != nil {
					t.Error(err)
				}
			} else {
				if err := testFrame.frame.Validate(ClientFrame); err != nil {
					t.Error(err)
				}
			}
		})
	}
}

func TestFrame_String(t *testing.T) {
	expected := `
===
CONNECT
accept-version:1.0,1.1,2.0
heart-beat:0,0
host:stomp.example.com
login:peter@parker.com
passcode:maryjane

<NUL>
===
`
	actual := fmt.Sprint(deserializedFrames[0].frame)
	if !cmp.Equal(actual, expected) {
		t.Error(cmp.Diff(actual, expected))
	}
}

func TestCheckValidEscapes(t *testing.T) {
	tests := []struct {
		str    string
		result bool
	}{
		{`\ngodhksdfs\t`, false},
		{`godhksdfs\`, false},
		{`\\\`, false},
		{`\\\n\r\c`, true},
	}

	for _, test := range tests {
		t.Run("str='"+test.str+"'", func(t *testing.T) {
			if checkValidEscapes(test.str) != test.result {
				t.Error(test)
			}
		})
	}
}
