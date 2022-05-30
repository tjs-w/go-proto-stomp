package main

import (
	"flag"
	"fmt"

	"github.com/tjs-w/go-proto-stomp/pkg/stomp"
)

func main() {
	transport := flag.String("t", "tcp", "transport for STOMP (tcp, websocket)")
	flag.Parse()
	host := "localhost"
	port := stomp.DefaultPort
	if "" != flag.Arg(0) {
		host = flag.Arg(0)
	}
	if "" != flag.Arg(1) {
		port = flag.Arg(1)
	}
	if *transport == "tcp" {
		if err := stomp.StartTcpBroker(host, port); err != nil {
			fmt.Println(err.Error())
		}
	}
	flag.Usage()
}
