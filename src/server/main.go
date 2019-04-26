package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net"
	"strings"

	netutils "github.com/cyk451/hole-punching/src/net-utils"
)

type Peer struct {
	*netutils.Client
	Writer *ClientWriter
}

/* use store interfaces */

/* keys are public ips, which should be unique */
var peerTable = map[string]*Peer{}

type ClientWriter struct {
	addr    net.UDPAddr
	conn    *net.UDPConn
	encoder *gob.Encoder
	buffer  bytes.Buffer
}

var _ io.Writer = &ClientWriter{}

func (c *ClientWriter) Write(in []byte) (w int, err error) {
	log.Println("Writing: ", string(in))
	// c.buffer = append(c.buffer, in...)
	return c.buffer.Write(in)
}

func (c *ClientWriter) Flush() {
	_, err := c.conn.WriteToUDP(c.buffer.Bytes(), &c.addr)
	if err != nil {
		log.Println("Write udp error: ", err)
	}
	c.buffer.Reset()
	return
}

func (c *ClientWriter) WriteSerialized(obj interface{}) error {
	defer c.Flush()
	return c.encoder.Encode(obj)
}

func NewClientWriter(conn *net.UDPConn, addr *net.UDPAddr) *ClientWriter {
	c := &ClientWriter{}
	c.conn = conn
	c.addr = *addr
	c.encoder = gob.NewEncoder(c)
	return c
}

func handleList(public string, conn *ClientWriter) {
	for i := range peerTable {
		if i == public {
			continue
		}
		log.Println("sending ", peerTable[i].Client)
		err := conn.WriteSerialized(peerTable[i].Client)
		if err != nil {
			log.Println("Writeclient: ", err)
			return
		}
	}
}

func hasPeer(public string) bool {
	_, ok := peerTable[public]
	return ok
}

func notifyNewPeer(peer *Peer) error {
	for i := range peerTable {
		if i == peer.Public {
			// don't tell your self
			continue
		}
		// will this call even timeout??
		log.Println("Notifying ", peerTable[i].Public, " ", peer.Public)
		err := peerTable[i].Writer.WriteSerialized(peer.Client)
		if err != nil {
			// unlikely to rollback...
			return err
		}
	}
	return nil
}

type IdGenerator = func() uint

// Note it count from 1
var idGen = func() IdGenerator {
	i := uint(0)
	return func() uint {
		i += 1
		return i
	}
}()

func handleRegisteration(public string, private string, conn *ClientWriter) {

	if hasPeer(public) {
		log.Println("This public ip is already registered. Do nothing.")
		return
	}

	temp := strings.Split(public, ":")
	if len(temp) != 2 {
		log.Println("public ip not having a port?")
		return
	}

	log.Println("Adding ", public, " to peer list")

	peer := &Peer{
		Client: &netutils.Client{
			Public:  public,
			Private: private,
			Id:      idGen(),
		},
		Writer: conn,
	}

	// adding semephore
	handleList(public, conn)

	notifyNewPeer(peer)

	peerTable[public] = peer

	log.Println("We now have ", len(peerTable), " clients")
}

func handleExit(public string) {
	delete(peerTable, public)
	// TODO implement client interface to remove peer
}

func main() {
	fmt.Println("You're running a server for hole punching p2p")
	addr := net.UDPAddr{
		Port: 11711,
		IP:   net.ParseIP("0.0.0.0"),
	}
	udp, err := net.ListenUDP("udp", &addr)
	if err != nil {
		log.Println("error listening: ", err)
		return
	}

	for {
		what := make([]byte, 512)
		c, public, err := udp.ReadFromUDP(what)
		if err != nil {
			if c == 0 {
				log.Println("udp accept error len ", c)
			}
			log.Println("udp error ", err)
			continue
		}

		var client *ClientWriter
		peer, ok := peerTable[public.String()]
		if !ok {
			client = NewClientWriter(udp, public)
		} else {
			client = peer.Writer
		}

		cmds := strings.Split(string(what[:c]), " ")
		log.Println("msg ", cmds)

		switch cmds[0] {
		case "register":

			// register <private ip address>
			// register itself to be a p2p client, respond a client struct
			// which is registered client
			if len(cmds) < 2 {
				log.Println("command register require peer private ip address...")
				continue
			}
			go handleRegisteration(
				public.String(),
				cmds[1], // private ip
				client,
			)
		case "close":
			go handleExit(
				public.String(),
			)
		default:
			log.Println("Unknown command from ", public.String(), ": ", cmds[0])
		}
	}
}
