package stomp

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"nhooyr.io/websocket"
)

// startWebsocketBroker is starts the STOMP broker on Websocket server
func startWebsocketBroker(host, port string, loginFunc LoginFunc) error {
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
			go NewSessionHandler(conn, loginFunc).Start()
		}
	})

	httpServer := &http.Server{
		Addr:    host + ":" + port,
		Handler: broker,
	}

	// Handle sigterm and await termChan signal
	termChan := make(chan os.Signal)
	signal.Notify(termChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-termChan // Blocks here until interrupted
		log.Println("Shutdown initiated ...")
		_ = httpServer.Shutdown(context.Background())
		wgSessions.Wait()
	}()

	if err := httpServer.ListenAndServe(); err != nil {
		return err
	}

	return nil
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
