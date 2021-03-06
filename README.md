# go-proto-stomp
[![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/tjs-w/go-proto-stomp)](https://github.com/tjs-w/go-proto-stomp/releases/latest)
[![go workflow](https://github.com/tjs-w/go-proto-stomp/actions/workflows/go.yml/badge.svg)](https://github.com/tjs-w/go-proto-stomp/actions/workflows/go.yml)
[![codeQL workflow](https://github.com/tjs-w/go-proto-stomp/actions/workflows/codeql-analysis.yml/badge.svg)](https://github.com/tjs-w/go-proto-stomp/actions/workflows/codeql-analysis.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/tjs-w/go-proto-stomp)](https://goreportcard.com/report/github.com/tjs-w/go-proto-stomp)
[![Maintainability](https://api.codeclimate.com/v1/badges/58623b1a2249bc679d9b/maintainability)](https://codeclimate.com/github/tjs-w/go-proto-stomp/maintainability)
[![Test Coverage](https://api.codeclimate.com/v1/badges/58623b1a2249bc679d9b/test_coverage)](https://codeclimate.com/github/tjs-w/go-proto-stomp/test_coverage)
[![codecov](https://codecov.io/gh/tjs-w/go-proto-stomp/branch/main/graph/badge.svg?token=655YSPLWCF)](https://codecov.io/gh/tjs-w/go-proto-stomp)
![GitHub](https://img.shields.io/github/license/tjs-w/go-proto-stomp)

[<img width=40% src=https://stomp.github.io/images/project-logo.png>](https://stomp.github.io/index.html)

_**S**imple **T**ext **O**riented **M**essaging **P**rotocol_


## STOMP Protocol Implementation in Golang (with interactive CLI)
[![Go Reference](https://pkg.go.dev/badge/github.com/tjs-w/go-proto-stomp/.svg)](https://pkg.go.dev/github.com/tjs-w/go-proto-stomp/)
[![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/tjs-w/go-proto-stomp)](https://github.com/tjs-w/go-proto-stomp/releases/latest)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/tjs-w/go-proto-stomp)

Includes:
1. `stomper`: Interactive CLI for STOMP Client
2. `stompd`: STOMP Broker
3. `stomp`: STOMP Broker/Client Library

### Supported Transports
1. TCP
2. Websocket

## stomper

```shell
stomper -t tcp
```

![stomper demo](stomper.gif "stomper")

## stompd
Starting the STOMP Server/Broker:
```shell
stompd -t tcp <host> <port>
```

## stomp
Fetching the module:
```shell
go get -u github.com/tjs-w/go-proto-stomp
```
Importing the package:
```shell
import "github.com/tjs-w/go-proto-stomp/pkg/stomp"
```

## STOMP Library Documentation
*[GoDoc](https://pkg.go.dev/github.com/tjs-w/go-proto-stomp/pkg/stomp)* lists the APIs for integrating both the
STOMP Broker and Client.
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
_#stomp #protocol #broker #server #client #cli_

## License
MIT License

Copyright (c) 2022 Tejas Wanjari
