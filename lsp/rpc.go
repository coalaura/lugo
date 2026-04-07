package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

var contentLengthPrefix = []byte("Content-Length: ")

func ReadMessage(r *bufio.Reader) ([]byte, error) {
	var length int

	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			return nil, err
		}

		if len(line) <= 2 && line[0] == '\r' {
			break
		}

		if bytes.HasPrefix(line, contentLengthPrefix) {
			valBytes := bytes.TrimSpace(line[len(contentLengthPrefix):])

			for _, b := range valBytes {
				if b >= '0' && b <= '9' {
					length = length*10 + int(b-'0')
				}
			}
		}
	}

	if length == 0 {
		return nil, fmt.Errorf("missing content length")
	}

	if length > 100*1024*1024 { // 100MB hard limit
		return nil, fmt.Errorf("message too large: %d bytes", length)
	}

	content := make([]byte, length)

	_, err := io.ReadFull(r, content)
	if err != nil {
		return nil, err
	}

	return content, nil
}

func WriteMessage(w io.Writer, msg any) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))

	_, err = w.Write([]byte(header))
	if err != nil {
		return err
	}

	_, err = w.Write(body)
	if err != nil {
		return err
	}

	return nil
}
