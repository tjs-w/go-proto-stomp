package stomp

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"strconv"
	"strings"
)

func frameSplitter(data []byte, _ bool) (advance int, token []byte, e error) {
	if len(data) == 0 {
		return 0, nil, nil
	}

	bodyLen := -1
	frameStart, start, end := 0, 0, 0

	// Ignore any \r\n that followed previous frame
	for data[start] == byte('\n') || data[start] == byte('\r') {
		start++
		if start == len(data) {
			return 0, nil, nil
		}
	}
	frameStart = start

	// STOMP command
	offset := bytes.IndexByte(data[start:], lineFeed)
	if offset == -1 {
		return 0, nil, errorMsg(errFrameScanner, "Too long, cannot find '\n'")
	}
	start = start + offset + 1

	// STOMP headers
	for {
		offset = bytes.IndexByte(data[start:], lineFeed)
		if offset == -1 {
			return 0, nil, errorMsg(errFrameScanner, "Too long, cannot find '\n'")
		}
		end = start + offset

		line := string(data[start:end])

		// Detect end of headers and break
		if line == "" {
			start++
			if start == len(data) {
				return 0, nil, nil
			}
			break
		}

		// Parse the Header line
		hdr := strings.Split(line, ":")
		if len(hdr) == 1 { // Missing ":"
			return 0, nil, errorMsg(errFrameScanner, "Expected ':' missing in header")
		}

		// Check for `content-length`
		if string(HdrKeyContentLength) == strings.ToLower(hdr[0]) {
			var err error
			bodyLen, err = strconv.Atoi(hdr[1])
			if err != nil {
				return 0, nil, errorMsg(errFrameScanner, "Invalid header: "+line+": "+err.Error())
			}
		}

		start = end + 1
	}

	// STOMP body
	if bodyLen == -1 {
		offset = bytes.IndexByte(data[start:], nullOctet)
		if offset == -1 {
			return 0, nil, nil
		}
		end = start + offset + 1
	} else {
		end = start + bodyLen
		if end > len(data)-1 {
			return 0, nil, nil
		}
		if data[end] != nullOctet {
			return 0, nil, errorMsg(errFrameScanner, "Invalid content-length, frame does end with NUL")
		}
		end++
	}

	// NUL may follow with newlines
	if end < len(data) && (data[end] == byte('\r') || data[end] == lineFeed) {
		end++
	}

	return end, data[frameStart:end], nil
}

// frameScanner reads from the reader and splits the byte-stream into chucks around the Frame delimiter/length.
// These chunks are then sent over the returned channel.
func frameScanner(conn io.Reader) <-chan []byte {
	scanner := bufio.NewScanner(conn)
	scanner.Split(frameSplitter)

	ch := make(chan []byte)
	go func() {
		for scanner.Scan() {
			ch <- scanner.Bytes()
		}
		if err := scanner.Err(); err != nil {
			log.Println(errFrameScanner, "frameScanner finished:", err)
		}
		close(ch)
	}()

	return ch
}
