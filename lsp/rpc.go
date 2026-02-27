package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

const ContentLengthPrefix = "Content-Length: "

func ReadMessage(r *bufio.Reader) ([]byte, error) {
	var length int

	for {
		header, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}

		if header == "\r\n" {
			break
		}

		if len(header) > len(ContentLengthPrefix) && header[:len(ContentLengthPrefix)] == ContentLengthPrefix {
			lengthStr := header[len(ContentLengthPrefix) : len(header)-2]

			length, err = strconv.Atoi(lengthStr)
			if err != nil {
				return nil, err
			}
		}
	}

	if length == 0 {
		return nil, fmt.Errorf("missing content length")
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
