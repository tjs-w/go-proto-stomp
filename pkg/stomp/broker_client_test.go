package stomp

import (
	"errors"
	"fmt"
	"io/ioutil"
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

func init() {
	log.SetOutput(ioutil.Discard)
}

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

			t.Log("Version:", ReleaseVersion())

			c := NewClientHandler(test.transport, "localhost", test.port, &ClientOpts{
				VirtualHost:              "",
				Login:                    "admin",
				Passcode:                 "9a$$w0rd",
				HeartbeatSendInterval:    3,
				HeartbeatReceiveInterval: 3,
			})

			c.SetMessageHandler(func(message *UserMessage) {
				id, err := strconv.Atoi(string(message.Body))
				if err != nil {
					t.Error()
				}
				if err := validateCustomTestHeader(message.Headers, id); err != nil {
					t.Error(err)
				}
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
			if err = sendErrorFrame(test.transport, test.port); err != nil {
				t.Error(err)
			}

			// Wrong login
			if err = failedLogin(test.transport, test.port); err != nil {
				t.Error(err)
			}

			// Wrong Heartbeat
			if err = sendWrongHeartbeat(test.transport, test.port); err != nil {
				t.Error(err)
			}
		})
	}

	tcp.Shutdown()
	wss.Shutdown()
}

func failedLogin(transport Transport, port string) error {
	cx := NewClientHandler(transport, "localhost", port, nil)
	if err := cx.Connect(true); err != nil {
		return err
	}
	if err := cx.Disconnect(); err != nil {
		return err
	}
	return nil
}

func sendWrongHeartbeat(transport Transport, port string) error {
	c := NewClientHandler(transport, "localhost", port, nil)
	headers := map[Header]string{
		HdrKeyHost:          "host",
		HdrKeyAcceptVersion: "1.2",
		HdrKeyHeartBeat:     "000",
		HdrKeyLogin:         "admin",
		HdrKeyPassCode:      "9a$$w0rd",
	}
	if err := c.send(CmdConnect, headers, []byte{}); err != nil {
		return err
	}

	c = NewClientHandler(transport, "localhost", port, nil)
	headers = map[Header]string{
		HdrKeyHost:          "host",
		HdrKeyAcceptVersion: "1.2",
		HdrKeyHeartBeat:     "A,B",
		HdrKeyLogin:         "admin",
		HdrKeyPassCode:      "9a$$w0rd",
	}
	if err := c.send(CmdConnect, headers, []byte{}); err != nil {
		return err
	}

	c = NewClientHandler(transport, "localhost", port, nil)
	headers = map[Header]string{
		HdrKeyHost:          "host",
		HdrKeyAcceptVersion: "1.2",
		HdrKeyHeartBeat:     "7,B",
		HdrKeyLogin:         "admin",
		HdrKeyPassCode:      "9a$$w0rd",
	}
	if err := c.send(CmdConnect, headers, []byte{}); err != nil {
		return err
	}

	c = NewClientHandler(transport, "localhost", port, nil)
	headers = map[Header]string{
		HdrKeyHost:          "host",
		HdrKeyAcceptVersion: "1.2.999",
		HdrKeyHeartBeat:     "1,8",
		HdrKeyLogin:         "admin",
		HdrKeyPassCode:      "9a$$w0rd",
	}
	if err := c.send(CmdConnect, headers, []byte{}); err != nil {
		return err
	}

	return nil
}

func sendErrorFrame(transport Transport, port string) error {
	c := NewClientHandler(transport, "localhost", port, nil)
	if err := c.Connect(true); err != nil {
		return err
	}
	f := NewFrame("WRONG_HEADER", nil, nil)
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
		return errors.New("authN denied")
	}

	ready := make(chan struct{})

	go func() {
		var err error
		if tcp, err = StartBroker(&BrokerOpts{
			Transport:                    TransportTCP,
			Host:                         "localhost",
			Port:                         "9990",
			LoginFunc:                    loginFunc,
			HeartbeatSendIntervalMsec:    2,
			HeartbeatReceiveIntervalMsec: 2,
		}); err != nil {
			log.Println(err)
			return
		}
		ready <- struct{}{}
		tcp.ListenAndServe()
	}()

	go func() {
		var err error
		if wss, err = StartBroker(&BrokerOpts{
			Transport:                    TransportWebsocket,
			Host:                         "localhost",
			Port:                         "9991",
			LoginFunc:                    loginFunc,
			HeartbeatSendIntervalMsec:    0,
			HeartbeatReceiveIntervalMsec: 0,
		}); err != nil {
			log.Println(err)
		}
		ready <- struct{}{}
		wss.ListenAndServe()
	}()

	<-ready
	<-ready

	ret := m.Run()
	os.Exit(ret)
}
