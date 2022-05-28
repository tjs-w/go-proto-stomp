# go-proto-stomp
[![Go Reference](https://pkg.go.dev/badge/github.com/tjs-w/go-proto-stomp/.svg)](https://pkg.go.dev/github.com/tjs-w/go-proto-stomp/)
![go workflow](https://github.com/tjs-w/go-proto-stomp/actions/workflows/go.yml/badge.svg)
![codeQL workflow](https://github.com/tjs-w/go-proto-stomp/actions/workflows/codeql-analysis.yml/badge.svg)
## STOMP Protocol Implementation in Golang (with interactive CLI)

Includes:
1. `stomper`: Interactive CLI for STOMP Client
2. `stompd`: STOMP Broker
3. `stomp`: STOMP Broker/Client Library

## stomper

```shell
stomper -p tcp
```

![stomper demo](stomper.gif "stomper")

## stompd
```shell
stompd -p tcp <host> <port>
```

## stomp
Package import:
```shell
go get -u github.com/tjs-w/go-proto-stomp
```
```shell
import "github.com/tjs-w/go-proto-stomp/pkg/stomp"
```

## STOMP Library Documentation
*[Go Doc](https://pkg.go.dev/github.com/tjs-w/go-proto-stomp@v0.1.0/pkg/stomp)*


## **[STOMP Protocol Specification](https://stomp.github.io/stomp-specification-1.2.html)**
The implementation adheres to the spec leaning towards the _version 1.2_ of the protocol.
### STOMP Frame: Augmented BNF Form
This implementation strictly follows the below grammar for frame construction and validation.
```
NULL                = <US-ASCII null (octet 0)>
LF                  = <US-ASCII line feed (aka newline) (octet 10)>
CR                  = <US-ASCII carriage return (octet 13)>
EOL                 = [CR] LF 
OCTET               = <any 8-bit sequence of data>

frame-stream        = 1*frame

frame               = command EOL
                      *( header EOL )
                      EOL
                      *OCTET
                      NULL
                      *( EOL )

command             = client-command | server-command

client-command      = "SEND"
                      | "SUBSCRIBE"
                      | "UNSUBSCRIBE"
                      | "BEGIN"
                      | "COMMIT"
                      | "ABORT"
                      | "ACK"
                      | "NACK"
                      | "DISCONNECT"
                      | "CONNECT"
                      | "STOMP"

server-command      = "CONNECTED"
                      | "MESSAGE"
                      | "RECEIPT"
                      | "ERROR"

header              = header-name ":" header-value
header-name         = 1*<any OCTET except CR or LF or ":">
header-value        = *<any OCTET except CR or LF or ":">
```
## License
MIT License

Copyright (c) 2022 Tejas Wanjari
