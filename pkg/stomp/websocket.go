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
	wgSessions *sync.WaitGroup
}

// startWebsocketBroker is starts the STOMP broker on Websocket server
func startWebsocketBroker(host, port string, loginFunc LoginFunc) (*wssBroker, error) {
	wss := &wssBroker{}
	broker := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for {
			c, err := websocket.Accept(w, r,
				&websocket.AcceptOptions{
					Subprotocols: []string{"v12.stomp"},
				})
			if err != nil {
				log.Println(err)
				return
			}
			log.Println(r.Context())

			conn := websocket.NetConn(context.Background(), c, websocket.MessageText)
			wss.wgSessions = &sync.WaitGroup{}
			go NewSession(conn, loginFunc, wss.wgSessions).Start()
		}
	})

	wss.httpServer = &http.Server{
		Addr:    host + ":" + port,
		Handler: broker,
	}
	wss.wgSessions = &sync.WaitGroup{}

	// Handle sigterm and await termChan signal
	termChan := make(chan os.Signal, 2)
	signal.Notify(termChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-termChan // Blocks here until interrupted
		wss.Shutdown()
	}()

	return wss, nil
}

func (wss *wssBroker) ListenAndServe() {
	if err := wss.httpServer.ListenAndServe(); err != nil {
		log.Println(err)
		return
	}
}

func (wss *wssBroker) Shutdown() {
	log.Println("Shutdown initiated ...")
	wss.wgSessions.Wait()
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
