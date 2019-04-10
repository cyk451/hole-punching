package netutils

import (
	"encoding/gob"
	"io"
)

type Message struct {
	who Client
	cmd string
}

type Client struct {
	Public  string // net.IP
	Private string // net.IP
	Id      uint
}

type Header struct {
	Num int
}

func WriteClient(conn io.Writer, c Client) error {
	encoder := gob.NewEncoder(conn)

	err := encoder.Encode(c)
	if err != nil {
		return err
	}
	return nil
}

func ReadClient(conn io.Reader, c *Client) error {
	decoder := gob.NewDecoder(conn)

	err := decoder.Decode(c)
	if err != nil {
		return err
	}

	return nil
}
