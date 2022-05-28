package main

import (
	"fmt"

	"github.com/tjs-w/go-proto-stomp/pkg/stomp"
)

func main() {
	if err := stomp.StartTcpBroker("localhost", "8888"); err != nil {
		fmt.Println(err.Error())
	}
}
