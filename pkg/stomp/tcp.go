package stomp

import (
	"net"
)

const (
	DefaultPort = "61613"
)

var listener net.Listener

// StartTcpBroker is the entry point for starting a TCP server for STOMP broker
func StartTcpBroker(host, port string) error {
	var err error
	// Listen for incoming connections.
	listener, err = net.Listen("tcp", host+":"+port)
	if err != nil {
		return errorMsg(errNetwork, "Listening failed: "+err.Error())
	}

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

// StopTcpBroker closes the TCP connection and turns off the server
func StopTcpBroker() {
	listener.Close()
}

// StartTcpClient is an entry point for connecting to the TCP STOMP broker. Apart from the hostname and port for
// the TCP server, it also accepts a function of type MessageHandlerFunc (defined as `func(message *UserMessage)`).
// This function definition is provided by the user and servers as a callback that processes every MESSAGE
// received from the STOMP broker.
func StartTcpClient(host, port string, fn MessageHandlerFunc) (*ClientHandler, error) {
	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		return nil, errorMsg(errNetwork, "Connect failed: "+err.Error())
	}
	c := NewClientHandler(conn, &ClientOpts{
		HeartBeat:      [2]int{0, 0},
		Login:          "",
		Passcode:       "",
		Host:           host,
		MessageHandler: fn,
	})
	return c, nil
}
