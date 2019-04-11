package netutils

import (
	"encoding/gob"
	"io"
	"log"

	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

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

func (c Client) MarshalBinary() (data []byte, err error) {
	data, err = json.Marshal(&c)
	log.Println("Marshalled: ", string(data))
	return
}

func (c *Client) UnmarshalBinary(data []byte) (err error) {
	log.Println("Unmarshalling: ", string(data))
	err = json.Unmarshal(data, &c)
	return
}
