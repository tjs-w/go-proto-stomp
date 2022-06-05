package stomp

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"
)

var (
	tcp Broker
	wss Broker
)

func beginCommitTx(t *testing.T, c *ClientHandler, dest string, testValidateID int) {
	tx, err := c.BeginTransaction()
	if err != nil {
		t.Error(err)
	}

	if err = tx.Send(dest, []byte(strconv.Itoa(testValidateID)), "plain/text", customTestHeader(testValidateID)); err != nil {
		t.Error(err)
	}

	if err = tx.CommitTransaction(); err != nil {
		t.Error(err)
	}
}

func beginAbortTx(t *testing.T, c *ClientHandler, dest string, testValidateID int) {
	tx, err := c.BeginTransaction()
	if err != nil {
		t.Error(err)
	}

	if err = tx.Send(dest, []byte(strconv.Itoa(testValidateID)), "plain/text", customTestHeader(testValidateID)); err != nil {
		t.Error(err)
	}

	if err = tx.AbortTransaction(); err != nil {
		t.Error(err)
	}
}

func TestStartBroker(t *testing.T) {
	tests := []struct {
		transport Transport
		port      string
	}{
		{TransportTCP, "9990"},
		{TransportWebsocket, "9991"},
	}
	for _, test := range tests {
		t.Run(string(test.transport), func(t *testing.T) {
			var err error
			dest := "/queue/foo"
			testValidateID := 0

			c := NewClientHandler(test.transport, "localhost", test.port, &ClientOpts{
				Host:      "",
				Login:     "admin",
				Passcode:  "9a$$w0rd",
				HeartBeat: [2]int{},
				MessageHandler: func(message *UserMessage) {
					id, err := strconv.Atoi(string(message.Body))
					if err != nil {
						t.Error()
					}
					if err := validateCustomTestHeader(message.Headers, id); err != nil {
						t.Error(err)
					}
				},
			})
			if err = c.Connect(false); err != nil {
				t.Error(err)
			}

			var subs *Subscription
			if subs, err = c.Subscribe(dest, ""); err != nil {
				t.Error(err)
			}

			if err = c.Send(dest, []byte(strconv.Itoa(testValidateID)), "plain/text",
				customTestHeader(testValidateID)); err != nil {
				t.Error(err)
			}
			testValidateID++

			beginCommitTx(t, c, dest, testValidateID)
			testValidateID++

			beginAbortTx(t, c, dest, testValidateID)

			if err = subs.Unsubscribe(); err != nil {
				t.Error(err)
			}

			if err = c.Disconnect(); err != nil {
				t.Error(err)
			}

			// Error frame
			if err = sendErrorFrame(t, test.transport, test.port); err != nil {
				t.Error(err)
			}

			// Wrong login
			if err = failedLogin(t, test.transport, test.port); err != nil {
				t.Error(err)
			}
		})
	}

	tcp.Shutdown()
	wss.Shutdown()
}

func failedLogin(t *testing.T, transport Transport, port string) error {
	cx := NewClientHandler(transport, "localhost", port, nil)
	if err := cx.Connect(true); err != nil {
		return err
	}
	if err := cx.Disconnect(); err != nil {
		return err
	}
	return nil
}

func sendErrorFrame(t *testing.T, transport Transport, port string) error {
	c := NewClientHandler(transport, "localhost", port, nil)
	if err := c.Connect(true); err != nil {
		return err
	}
	f := NewFrame(Command("WRONG_HEADER"), nil, nil)
	if _, err := c.conn.Write(f.Serialize()); err != nil {
		return err
	}
	return nil
}

func customTestHeader(id int) map[string]string {
	return map[string]string{
		"testValidateID": strconv.Itoa(id),
	}
}

func validateCustomTestHeader(h map[string]string, id int) error {
	for k, v := range h {
		if k == strings.ToLower("testValidateID") {
			if v != strconv.Itoa(id) {
				return fmt.Errorf("testValidateID: expected=%d, got=%s\n", id, v)
			}
			return nil
		}
	}
	return fmt.Errorf("Missing testValidateID: %d, headers=%v\n", id, h)
}

func TestMain(m *testing.M) {
	loginFunc := func(login, passcode string) error {
		if login == "admin" && passcode == "9a$$w0rd" {
			return nil
		}
		return errors.New("authentication failed")
	}

	go func() {
		var err error
		if tcp, err = StartBroker(TransportTCP, "localhost", "9990", loginFunc); err != nil {
			log.Println(err)
			return
		}
		tcp.ListenAndServe()
	}()

	go func() {
		var err error
		if wss, err = StartBroker(TransportWebsocket, "localhost", "9991", loginFunc); err != nil {
			log.Println(err)
		}
		wss.ListenAndServe()
	}()

	ret := m.Run()
	os.Exit(ret)
}
