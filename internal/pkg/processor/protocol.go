package processor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/vaizmanai/srvisit/internal/pkg/common"
)

const maxMessageSize = 1 << 20

// Messages are adjacent JSON objects. Braces inside strings are payload data.
func readMessage(reader *bufio.Reader) (Message, error) {
	var message Message
	var frame []byte
	depth := 0
	quoted, escaped := false, false
	for size := 0; size < maxMessageSize; size++ {
		b, err := reader.ReadByte()
		if err != nil {
			return message, err
		}
		if depth == 0 && b != '{' {
			continue
		}
		frame = append(frame, b)
		if quoted {
			if escaped {
				escaped = false
			} else if b == '\\' {
				escaped = true
			} else if b == '"' {
				quoted = false
			}
			continue
		}
		switch b {
		case '"':
			quoted = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				if err := json.Unmarshal(frame, &message); err != nil {
					return message, fmt.Errorf("invalid message: %w", err)
				}
				return message, nil
			}
		}
	}
	return message, fmt.Errorf("message exceeds %d bytes", maxMessageSize)
}

type controlConn struct {
	net.Conn
	writeMutex sync.Mutex
}

func (conn *controlConn) Write(p []byte) (int, error) {
	conn.writeMutex.Lock()
	defer conn.writeMutex.Unlock()
	if err := conn.Conn.SetWriteDeadline(time.Now().Add(common.WriteTimeout * time.Second)); err != nil {
		return 0, err
	}
	return writeAll(conn.Conn, p)
}

func writeAll(writer io.Writer, p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		n, err := writer.Write(p)
		written += n
		p = p[n:]
		if err != nil {
			return written, err
		}
		if n == 0 {
			return written, io.ErrShortWrite
		}
	}
	return written, nil
}
