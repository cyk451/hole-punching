package main

import (
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net"

	netutils "github.com/cyk451/hole-punching/src/net-utils"
)

type p2pHandler interface {
	Handle(msg []byte) error
}

type P2PClient struct {
	conn *net.UDPConn
	// decoder *gob.Decoder
	hostUDP *net.UDPAddr
	// hostIo  HostIO
	peerIo  PeerIO
	localIP string

	self netutils.Client

	buffers map[string]io.Writer

	peerMessages chan Message
}

func newP2PConnection(host string) *P2PClient {
	hostUDP, err := net.ResolveUDPAddr("udp", host)
	if err != nil {
		fmt.Println("udp resolving error ", err)
	}

	// this give you a connected udp.
	conn, err := net.DialUDP("udp", nil, hostUDP)
	if err != nil {
		fmt.Println("Dial, ", err)
		return nil // err
	}

	localIP := conn.LocalAddr().String()

	conn.Close()

	// ok now we need an unconnected udp connection listening on the port we
	// just used.

	// listening peer connection
	udpAddr, err := net.ResolveUDPAddr("udp", localIP)
	if err != nil {
		log.Fatal(err)
		return nil
	}

	conn, err = net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal(err)
		return nil
	}

	c := &P2PClient{}
	c.hostUDP = hostUDP
	c.conn = conn
	c.localIP = localIP
	// c.hostIo = HostIO{c.conn}
	c.peerIo = PeerIO{c.conn}
	c.buffers = make(map[string]io.Writer)
	// c.decoder = gob.NewDecoder(c.hostIo)

	return c
}

func (s P2PClient) WriteHost(p []byte) (l int, e error) {
	return s.conn.WriteToUDP(p, s.hostUDP)
}

/*
func (s P2PClient) ReadHost(p []byte) (l int, e error) {
	return s.hostIo.Read(p)
}
*/

type WriteWrap struct {
	w io.Writer
}

func (w *WriteWrap) Write(p []byte) (i int, e error) {
	i, e = w.w.Write(p)
	log.Println("host bytes read ", string(p))
	return
}

func (s P2PClient) ConnectHost() (*HostIO, error) {

	what := "register " + s.localIP
	n, err := s.WriteHost([]byte(what))
	if err != nil {
		fmt.Println(err, ", n: ", n)
		return nil, err
	}

	/*
		err = s.ReadSerialized(&s.self)
		if err != nil {
			fmt.Println("reading client ", err)
			return nil, err
		}
		fmt.Printf("got assigned id %d\n", s.self.Id)
	*/

	// buf := bytes.NewBuffer(make([]byte, 1024))

	r, w := io.Pipe()

	host := &HostIO{
		decoder: gob.NewDecoder(r),
	}
	s.buffers[s.hostUDP.String()] = &WriteWrap{w}

	return host, nil
}

/*
func (s P2PClient) ReadSerialized(obj interface{}) (err error) {
	for retries := 10; retries > 0; retries-- {
		err = s.decoder.Decode(obj)
		// somehow we need to do a couple retries
		if err == nil {
			break
		}
	}
	return
}
*/

func (s P2PClient) handle(msg []byte, source string) {
	h, found := s.buffers[source]
	if !found {
		log.Println("msg from ", source, " has no handler: ", msg)
		return
	}

	log.Println("Some messages from ", source)

	_, err := h.Write(msg)
	if err != nil {
		log.Println("error handling msg from ", source, ": ", err)
	}
}

func (s P2PClient) Listen() {
	for {
		msg := make([]byte, 512)
		n, addr, err := s.conn.ReadFromUDP(msg)
		if err != nil {
			log.Println("read error: ", err)
			continue
		}

		go s.handle(msg[:n], addr.String())
	}
}
