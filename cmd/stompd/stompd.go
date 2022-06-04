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
	if "" != flag.Arg(0) {
		host = flag.Arg(0)
	}
	if "" != flag.Arg(1) {
		port = flag.Arg(1)
	}

	t := stomp.TransportTCP
	if *transport == "websocket" {
		t = stomp.TransportWebsocket
	}

	if err := stomp.StartBroker(t, host, port, nil); err != nil {
		log.Fatalln(err)
	}

	flag.Usage()
}
