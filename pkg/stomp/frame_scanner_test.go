package stomp

import (
	"bufio"
	"io/ioutil"
	"math/rand"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func readInChunks(t *testing.T, typ string) <-chan []byte {
	var err error
	var data []byte
	files, err := filepath.Glob("testdata/frames/" + typ + "/*.golden.txt")
	if err != nil {
		t.Error(err)
	}
	for _, f := range files {
		var bytes []byte
		bytes, err = ioutil.ReadFile(f)
		if err != nil {
			t.Error(err)
		}
		data = append(data, bytes...)
	}

	rand.Seed(time.Now().UnixNano())
	sz := 1 + rand.Intn(23)
	t.Log("Chunk size:", sz)

	ch := make(chan []byte)
	go func() {
		start, end := 0, sz
		for end < len(data) {
			ch <- data[start:end]

			start = end
			end += sz
			if end >= len(data) {
				end = len(data)
				ch <- data[start:end]
			}
		}
		close(ch)
	}()

	return ch
}

func client(t *testing.T, typ string) {
	conn, err := net.Dial("tcp", "0.0.0.0:9999")
	if err != nil {
		t.Error(err)
	}
	defer conn.Close()

	w := bufio.NewWriter(conn)
	ch := readInChunks(t, typ)

	for data := range ch {
		if _, err = w.Write(data); err != nil {
			t.Error(err)
		}
		if err = w.Flush(); err != nil {
			t.Error(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	conn.Close()
}

func server(t *testing.T) net.Listener {
	listener, err := net.Listen("tcp", "0.0.0.0:9999")
	if err != nil {
		t.Error(err)
	}
	return listener
}

func TestFrameScanner(t *testing.T) {
	listen := server(t)
	defer listen.Close()
	t.Parallel()

	// Test Client Commands
	testClientServer(t, listen, "client")

	// Test Server Commands
	testClientServer(t, listen, "server")
}

func testClientServer(t *testing.T, listen net.Listener, typ string) {
	go client(t, typ)

	conn, err := listen.Accept()
	defer func() {
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err != nil {
		t.Error(err)
	}

	for rawFrame := range frameScanner(conn) {
		t.Run(strings.Split(string(rawFrame), "\n")[0], func(t *testing.T) {
			frame := Frame{}
			if err = frame.Deserialize(rawFrame); err != nil {
				t.Error(err)
			}

			src := ClientFrame
			if typ == "server" {
				src = ServerFrame
			}

			if err = frame.Validate(src); err != nil {
				t.Error(err)
			}
		})
	}
}
