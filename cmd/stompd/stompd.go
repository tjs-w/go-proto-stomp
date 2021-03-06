package main

import (
	"flag"
	"log"

	"github.com/tjs-w/go-proto-stomp/pkg/stomp"
)

func main() {
	transport := flag.String("t", "websocket", "transport for STOMP (tcp, websocket)")
	flag.Parse()
	host := "localhost"
	port := stomp.DefaultPort
	if flag.Arg(0) != "" {
		host = flag.Arg(0)
	}
	if flag.Arg(1) != "" {
		port = flag.Arg(1)
	}

	t := stomp.TransportTCP
	if *transport == "websocket" {
		t = stomp.TransportWebsocket
	}

	var err error
	var broker stomp.Broker
	if broker, err = stomp.StartBroker(&stomp.BrokerOpts{
		Transport:                    t,
		Host:                         host,
		Port:                         port,
		LoginFunc:                    nil,
		HeartbeatSendIntervalMsec:    5000,
		HeartbeatReceiveIntervalMsec: 5000,
	}); err != nil {
		log.Fatalln(err)
	}
	broker.ListenAndServe()
	flag.Usage()
}
