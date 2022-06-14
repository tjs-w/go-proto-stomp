package stomp

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"nhooyr.io/websocket"
)

type wssBroker struct {
	httpServer *http.Server
}

// startWebsocketBroker is starts the STOMP broker on Websocket server
func startWebsocketBroker(opts *BrokerOpts) (*wssBroker, error) {
	wss := &wssBroker{}

	broker := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wgSessions := &sync.WaitGroup{}
		for {
			c, err := websocket.Accept(w, r,
				&websocket.AcceptOptions{
					Subprotocols: []string{"v12.stomp"},
				})
			if err != nil {
				log.Println(err)
				break
			}

			conn := websocket.NetConn(context.Background(), c, websocket.MessageText)
			wgSessions.Add(1)
			go NewSession(conn, opts.LoginFunc, wgSessions, opts.HeartbeatSendIntervalMsec,
				opts.HeartbeatReceiveIntervalMsec).Start()
		}
		wgSessions.Wait()
	})

	wss.httpServer = &http.Server{
		Addr:    opts.Host + ":" + opts.Port,
		Handler: broker,
	}

	// Handle sigterm and await termChan signal
	termChan := make(chan os.Signal, 2)
	signal.Notify(termChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-termChan // Blocks here until interrupted
		wss.Shutdown()
	}()

	return wss, nil
}

// ListenAndServe accepts the websocket client connections and servers the STOMP requests
func (wss *wssBroker) ListenAndServe() {
	if err := wss.httpServer.ListenAndServe(); err != nil {
		log.Println(err)
		return
	}
}

// Shutdown brings down the websocket server gracefully
func (wss *wssBroker) Shutdown() {
	log.Println("Shutdown initiated ...")
	if err := wss.httpServer.Shutdown(context.Background()); err != nil {
		log.Println(err)
	}
}

func startWebsocketClient(host, port string) (net.Conn, error) {
	c, _, err := websocket.Dial(context.Background(), "ws://"+host+":"+port,
		&websocket.DialOptions{
			Subprotocols: []string{"v12.stomp"},
		})
	if err != nil {
		return nil, err
	}

	return websocket.NetConn(context.Background(), c, websocket.MessageText), nil
}
