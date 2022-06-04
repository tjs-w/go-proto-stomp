package stomp

import (
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

const (
	DefaultPort = "61613"
)

type tcpBroker struct {
	listener   net.Listener
	wgSessions *sync.WaitGroup
	handler    func()
}

// startTcpBroker is the entry point for starting a TCP server for STOMP broker
func startTcpBroker(host, port string, loginFunc LoginFunc) (*tcpBroker, error) {
	var err error
	tcp := &tcpBroker{}

	// Listen for incoming connections.
	tcp.listener, err = net.Listen("tcp", host+":"+port)
	if err != nil {
		return nil, errorMsg(errNetwork, "Listening failed: "+err.Error())
	}
	tcp.wgSessions = &sync.WaitGroup{}

	// Handle sigterm and await termChan signal
	termChan := make(chan os.Signal, 2)
	signal.Notify(termChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-termChan // Blocks here until interrupted
		tcp.Shutdown()
	}()

	tcp.handler = func() {
		for {
			// Listen for an incoming connection.
			conn, err := tcp.listener.Accept()
			if err != nil {
				log.Println(err)
				return
			}
			// Handle connections in a new goroutine.
			go NewSessionHandler(conn, loginFunc, tcp.wgSessions).Start()
		}
	}

	return tcp, nil
}

func (tcp *tcpBroker) ListenAndServe() {
	tcp.handler()
}

func (tcp *tcpBroker) Shutdown() {
	log.Println("Shutdown initiated ...")
	tcp.wgSessions.Wait()
	_ = tcp.listener.Close()
}

func startTcpClient(host, port string) (net.Conn, error) {
	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		return nil, errorMsg(errNetwork, "Connect failed: "+err.Error())
	}
	return conn, nil
}
