package main

import (
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net"
	"strings"

	netutils "github.com/cyk451/hole-punching/src/net-utils"
)

type Peer struct {
	netutils.Client
	Writer *ClientWriter
}

/* TODO adding lock to this */
/* use store interfaces */

/* keys are public ips, which should be unique */
var peerTable = map[string]Peer{}

type ClientWriter struct {
	conn    *net.UDPConn
	addr    net.UDPAddr
	encoder *gob.Encoder
}

var _ io.Writer = &ClientWriter{}

func (c ClientWriter) Write(in []byte) (w int, err error) {
	log.Println("Writing: ", string(in))
	for w < len(in) {
		t := 0
		t, err = c.conn.WriteToUDP(in, &c.addr)
		w += t
		if err != nil {
			return
		}
	}
	return
}

func (c ClientWriter) WriteSerialized(obj interface{}) error {
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
	/*
		if !hasPeer(public) {
			fmt.Println("Register your private ip first")
			return
		}

			conn.WriteSerialized(netutils.Header{
				Num: len(peerTable) - 1,
			})
	*/

	for i := range peerTable {
		if i == public {
			continue
		}
		fmt.Printf("sending %+v\n", peerTable[i].Client)
		err := conn.WriteSerialized(peerTable[i].Client)
		// fmt.Println("End sending")
		if err != nil {
			fmt.Println("Writeclient: ", err)
			return
		}
		// fmt.Println("peer ", i, " Wrote")
	}
}

func hasPeer(public string) bool {
	_, ok := peerTable[public]
	return ok
}

func notifyNewPeer(peer Peer) error {
	for i := range peerTable {
		if i == peer.Public {
			// don't tell your self
			continue
		}
		// will this call even timeout??
		log.Println("Notifying ", peerTable[i].Public, " ", peer.Public)
		err := peerTable[i].Writer.WriteSerialized(peer)
		if err != nil {
			// unlikely to rollback...
			return err
		}
	}
	return nil
}

func handleRegisteration(public string, private string, conn *ClientWriter) {

	if hasPeer(public) {
		fmt.Println("This public ip is already registered. Do nothing.")
		return
	}

	temp := strings.Split(public, ":")
	if len(temp) != 2 {
		fmt.Println("public ip not having a port?")
		return
	}

	fmt.Println("Adding ", public, " to peer list")

	newClient := netutils.Client{
		Public:  public,
		Private: private,
		Id:      uint(len(peerTable)),
	}

	// responding peer his own information
	// err := netutils.WriteClient(conn, newClient)
	/*
		err := conn.WriteSerialized(newClient)
		if err != nil {
			fmt.Println("Writeclient: ", err)
			return
		}
	*/

	peer := Peer{
		Client: newClient,
		Writer: conn,
	}

	// adding semephore
	handleList(public, conn)

	notifyNewPeer(peer)

	peerTable[public] = peer

	fmt.Println("We now have ", len(peerTable), " clients")
}

func main() {
	fmt.Println("You're running a server for hole punching p2p")
	addr := net.UDPAddr{
		Port: 11711,
		IP:   net.ParseIP("0.0.0.0"),
	}
	udp, err := net.ListenUDP("udp", &addr)
	if err != nil {
		fmt.Println("error listening: ", err)
		return
	}

	for {
		what := make([]byte, 300)
		c, public, err := udp.ReadFromUDP(what)
		if err != nil {
			if c == 0 {
				fmt.Println("udp accept error len ", c)
			}
			fmt.Println("udp error ", err)
			continue
		}

		client := NewClientWriter(udp, public)

		cmds := strings.Split(string(what[:c]), " ")
		log.Println("msg ", cmds)

		switch cmds[0] {
		case "register":

			// register <private ip address>
			// register itself to be a p2p client, respond a client struct
			// which is registered client
			if len(cmds) < 2 {
				fmt.Println("command register require peer private ip address...")
				continue
			}
			go handleRegisteration(
				public.String(),
				cmds[1], // private ip
				client,
			)
			// case "list":
			// go handleList(public.String(), client)
		default:
			log.Println("Unknown command from ", public.String(), ": ", cmds[0])
		}
	}
	// log.Println("reach end of execution")
}
