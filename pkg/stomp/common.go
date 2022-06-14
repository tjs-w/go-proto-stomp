package stomp

//
// This file contains resources used by both the broker and the client.
//

import (
	_ "embed"
	"log"
	"time"

	"github.com/go-co-op/gocron"
)

const (
	DefaultPort = "61613"
)

// Transport represents the underlying transporting protocol for STOMP
type Transport string

const (
	TransportTCP       Transport = "TCP"       // STOMP over TCP
	TransportWebsocket Transport = "Websocket" // STOMP over Websocket
)

//go:generate sh -c "git describe --tags --abbrev=0 | tee version.txt"
//go:embed version.txt
var releaseVersion string

// Scheduler for sending heartbeats
var sched = gocron.NewScheduler(time.UTC)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

// ReleaseVersion returns the version of the go-proto-stomp module
func ReleaseVersion() string {
	return releaseVersion
}
