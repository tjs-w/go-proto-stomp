package stomp

import (
	"fmt"
	"sort"
	"strings"
)

const (
	nullOctet = byte('\x00')
	lineFeed  = byte('\n')
)

func serialize(f *Frame) []byte {
	sb := &strings.Builder{}

	// command
	sb.WriteString(fmt.Sprintf("%s\n", f.command))

	// headers
	if f.headers != nil {
		for _, hk := range f.getSortedHeaderKeys() {
			sb.WriteString(
				fmt.Sprintf("%s:%s\n", escape(hk), escape(f.headers[Header(hk)])),
			)
		}
	}

	// Indicator of end of headers
	sb.WriteString("\n")

	// body
	if f.body != nil {
		sb.Write(f.body)
	}

	// NUL
	sb.Write([]byte{nullOctet})

	return []byte(sb.String())
}

func (f *Frame) getSortedHeaderKeys() []string {
	keys := make([]string, 0, len(f.headers))
	for hk := range f.headers {
		keys = append(keys, string(hk))
	}
	sort.Strings(keys)
	return keys
}

func deserialize(buf []byte, f *Frame) error {
	// The frame must end at NUL but can have newlines following that.
	nulPos := 0
	for i := len(buf) - 1; i >= 0; i-- {
		if buf[i] == lineFeed {
			continue
		}
		if buf[i] == nullOctet {
			nulPos = i
			break
		}
		return errorMsg(errByteFormat, "Message frame must end with NUL byte")
	}
	// Truncate buf to exclude the NUL.
	// If this is skipped the newlines and the NUL will end up in the body.
	buf = buf[:nulPos]

	// Remove all carriage-returns
	s := strings.ReplaceAll(string(buf), "\r", "") // Ignore "\r" chars
	lines := strings.Split(s, "\n")                // Split at newline\\

	cmd := Command(lines[0])
	lines = lines[1:]

	// Extracting the headers & body
	headers := make(map[Header]string)
	for _, l := range lines {
		if l == "" {
			break
		}
		h := strings.Split(l, ":")
		if len(h) != 2 {
			return errorMsg(errByteFormat, "Header must contain only one ':', bad header: "+l)
		}
		k, v := Header(unescape(h[0])), unescape(h[1])
		headers[k] = v
	}

	// Fetch body
	i := strings.Index(s, "\n\n")
	if i == -1 {
		return errorMsg(errByteFormat, "End of headers must be marked by two newlines: \\n\\n")
	}
	var body []byte = nil
	if i+2 != len(buf) {
		body = []byte(s[i+2 : len(buf)])
	}

	// Fill
	f.command = cmd
	f.headers = headers
	f.body = body

	return nil
}

func unescape(l string) string {
	l = strings.ReplaceAll(l, `\\`, "\\")
	l = strings.ReplaceAll(l, `\n`, "\n")
	l = strings.ReplaceAll(l, `\c`, ":")
	return l
}

func escape(l string) string {
	l = strings.ReplaceAll(l, "\\", `\\`)
	l = strings.ReplaceAll(l, "\n", `\n`)
	l = strings.ReplaceAll(l, ":", `\c`)
	return l
}
