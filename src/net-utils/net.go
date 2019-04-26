package netutils

import (
	"encoding/gob"
	"io"
	"log"
	"net"

	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// sync
func initiateHandShake(conn *net.UDPConn, target string, timeout uint64) error {
	return nil
}

func handShakeResponder(raw []byte, source string) int {
	// err := AckMessage.UnmarshalBinary(raw)
	/*
		if err == nil {
			return len(raw)
		}
	*/

	return 0
}

type AckMessage struct {
	who Client
	cmd string
}

type Client struct {
	Public  string // net.IP
	Private string // net.IP
	Id      uint
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

func (c *Client) MarshalBinary() (data []byte, err error) {
	data, err = json.Marshal(c)
	log.Println("Marshalled: ", string(data))
	return
}

func (c *Client) UnmarshalBinary(data []byte) (err error) {
	log.Println("Unmarshalling: ", string(data))
	err = json.Unmarshal(data, c)
	return
}

/*
type UDPHandler func(ctx *UDPContext, raw []byte) error

type UDPRouter interface {
	AddUDPHandler()
	AddUDPHandlers()
}

type UDPServer struct {
	*net.UDPConn
	router UDPRouter
}

func (u *UDPServer) ListenAndServe() {
}
*/
