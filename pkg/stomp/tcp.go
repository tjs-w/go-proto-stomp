package stomp

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

const (
	DefaultPort = "61613"
)

// startTcpBroker is the entry point for starting a TCP server for STOMP broker
func startTcpBroker(host, port string, loginFunc LoginFunc) error {
	// Listen for incoming connections.
	listener, err := net.Listen("tcp", host+":"+port)
	if err != nil {
		return errorMsg(errNetwork, "Listening failed: "+err.Error())
	}

	// Handle sigterm and await termChan signal
	termChan := make(chan os.Signal)
	signal.Notify(termChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-termChan // Blocks here until interrupted
		log.Println("Shutdown initiated ...")
		listener.Close()
		wgSessions.Wait()
	}()

	for {
		// Listen for an incoming connection.
		conn, err := listener.Accept()
		if err != nil {
			return errorMsg(errNetwork, "Accepting failed: "+err.Error())
		}
		// Handle connections in a new goroutine.
		go NewSessionHandler(conn, nil).Start()
	}
}

func startTcpClient(host, port string) (net.Conn, error) {
	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		return nil, errorMsg(errNetwork, "Connect failed: "+err.Error())
	}
	return conn, nil
}
