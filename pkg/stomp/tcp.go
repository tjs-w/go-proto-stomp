package stomp

import (
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type tcpBroker struct {
	listener net.Listener
	handler  func()
}

// startTcpBroker is the entry point for starting a TCP server for STOMP broker
func startTcpBroker(opts *BrokerOpts) (*tcpBroker, error) {
	var err error
	tcp := &tcpBroker{}

	// Listen for incoming connections.
	tcp.listener, err = net.Listen("tcp", opts.Host+":"+opts.Port)
	if err != nil {
		return nil, errorMsg(errNetwork, "Listening failed: "+err.Error())
	}

	// Handle sigterm and await termChan signal
	termChan := make(chan os.Signal, 2)
	signal.Notify(termChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-termChan // Blocks here until interrupted
		tcp.Shutdown()
	}()

	tcp.handler = func() {
		wgSessions := &sync.WaitGroup{}
		for {
			// Listen for an incoming connection.
			conn, err := tcp.listener.Accept()
			if err != nil {
				log.Println(err)
				break
			}
			// Handle connections in a new goroutine.
			wgSessions.Add(1)
			go NewSession(conn, opts.LoginFunc, wgSessions, opts.HeartbeatSendIntervalMsec,
				opts.HeartbeatReceiveIntervalMsec).Start()
		}
		wgSessions.Wait()
	}

	return tcp, nil
}

// ListenAndServe accepts the TCP client connections and servers the STOMP requests
func (tcp *tcpBroker) ListenAndServe() {
	tcp.handler()
}

// Shutdown brings down the TCP server gracefully
func (tcp *tcpBroker) Shutdown() {
	log.Println("Shutdown initiated ...")
	_ = tcp.listener.Close()
}

func startTcpClient(host, port string) (net.Conn, error) {
	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		return nil, errorMsg(errNetwork, "Connect failed: "+err.Error())
	}
	return conn, nil
}
