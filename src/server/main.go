package main

import (
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"strings"

	netutils "github.com/cyk451/ttt-server/src/net-utils"
)

type Peer struct {
	netutils.Client
}

var peerTable = map[string]Peer{}

type ClientWriter struct {
	conn    *net.UDPConn
	addr    *net.Addr
	encoder *gob.Encoder
}

var _ io.Writer = &ClientWriter{}

func (c ClientWriter) Write(in []byte) (w int, err error) {
	for w < len(in) {
		t := 0
		t, err = c.conn.WriteTo(in, *c.addr)
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

func NewClientWriter(conn *net.UDPConn, addr *net.Addr) *ClientWriter {
	c := &ClientWriter{}
	c.conn = conn
	c.addr = addr
	c.encoder = gob.NewEncoder(c)
	return c
}

func handleList(public string, conn *ClientWriter) {
	if !hasPeer(public) {
		fmt.Println("Register your private ip first")
		return
	}

	conn.WriteSerialized(netutils.Header{
		Num: len(peerTable) - 1,
	})

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

func handleRegisteration(public string, private string, conn *ClientWriter) {

	if hasPeer(public) {
		fmt.Println("This public ip already registered. Do nothing.")
		return
	}

	temp := strings.Split(public, ":")
	if len(temp) != 2 {
		fmt.Println("public ip not having a port?")
		return
	}

	fmt.Println("Adding ", public, " to peer list")

	newClient := netutils.Client{
		Public:  public,                  // net.ParseIP(ipNPort[0]),
		Private: private + ":" + temp[1], //  net.ParseIP(private),
		Id:      uint(len(peerTable)),
	}

	// responding peer his own information
	// err := netutils.WriteClient(conn, newClient)
	err := conn.WriteSerialized(newClient)
	if err != nil {
		fmt.Println("Writeclient: ", err)
		return
	}

	peerTable[public] = Peer{
		Client: newClient,
	}

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
		c, public, err := udp.ReadFrom(what)
		if err != nil {
			if c == 0 {
				fmt.Println("udp accept error len ", c)
			}
			fmt.Println("udp error ", err)
			continue
		}
		cmds := strings.Split(string(what), " ")

		client := NewClientWriter(udp, &public)
		fmt.Println("msg ", cmds)
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
		case "list":
			go handleList(public.String(), client)
		default:
			fmt.Println("Unknown command: ", cmds[0])
		}
	}
	fmt.Println("reach end of execution")
}
