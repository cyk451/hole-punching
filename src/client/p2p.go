package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	netutils "github.com/cyk451/hole-punching/src/net-utils"
)

type P2PHandler func(source string, msg []byte) (processed int)

/*
type p2pIO interface {
	// Read([]byte) (int, error)

	Write([]byte) (int, error)

	// writeToReadPipe([]byte) (int, error)
}
*/

type LockedUDPConn struct {
	*net.UDPConn
	mutex sync.Mutex
}

type HostIO struct {
	conn *LockedUDPConn
	udp  *net.UDPAddr
	// decoder        *gob.Decoder
	readPipeWriter io.Writer
	io.Reader
	// udp            *net.UDPAddr
}

func (s *HostIO) Write(bytes []byte) (i int, e error) {
	s.conn.mutex.Lock()
	i, e = s.conn.WriteToUDP(bytes, s.udp)
	s.conn.mutex.Unlock()
	return i, e
}

func (s *HostIO) writeToReadPipe(bytes []byte) (int, error) {
	return s.readPipeWriter.Write(bytes)
}

func (s *HostIO) ReadSerialized(obj interface{}) (err error) {
	for retries := 10; retries > 0; retries-- {
		// err = s.decoder.Decode(obj)
		// somehow we need to do a couple retries
		if err == nil {
			return
		}
	}
	return
}

type P2PClient struct {
	conn         *LockedUDPConn
	hostUDP      *net.UDPAddr
	localIP      string
	peerList     []netutils.Client
	peerMessages chan []byte
	ios          map[string]io.Writer
	self         netutils.Client
	handlers     []P2PHandler
	newHandlers  []P2PHandler
}

func (p2p *P2PClient) WriteToPeers(bytes []byte) (err error) {
	for _, t := range p2p.ios {
		_, ok := t.(*PeerIO)
		if !ok {
			continue
		}
		_, err = t.Write(bytes)
		if err != nil {
			log.Println("Write to peers: ", err)
		}
	}
	return err
}

func newP2PClient(host string) *P2PClient {
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
	c.conn = &LockedUDPConn{UDPConn: conn}
	c.localIP = localIP
	c.ios = make(map[string]io.Writer)

	return c
}

func (s *P2PClient) AddSourcedHandler(expecting string, pureHandler func([]byte) int) {
	s.AddHandler(
		func(source string, raw []byte) int {
			if source != expecting {
				return 0
			}
			return pureHandler(raw)
		},
	)
}

func (s *P2PClient) AddHandler(h P2PHandler) {
	s.newHandlers = append(s.newHandlers, h)
}

func (s *P2PClient) Register() error {
	host, ok := s.ios[s.hostUDP.String()]
	if !ok {
		return errors.New("Host not connected")
	}

	what := "register " + s.localIP
	n, err := host.Write([]byte(what))
	if err != nil {
		fmt.Println(err, ", n: ", n)
		// return nil, err
		return err
	}
	return nil
}

func (s *P2PClient) ConnectHost() (*HostIO, error) {
	// r, w := io.Pipe()

	host := &HostIO{
		// decoder: gob.NewDecoder(),
		// Reader:         r,
		// readPipeWriter: w,
		udp:  s.hostUDP,
		conn: s.conn,
	}
	s.ios[s.hostUDP.String()] = host

	go s.listen()

	return host, nil
}

type Msg struct {
	source *net.UDPAddr
	raw    []byte
}

func (s *P2PClient) handlerThread(ch chan Msg) {
	for {
		msg := <-ch

		if len(s.newHandlers) != 0 {
			s.handlers = append(s.handlers, s.newHandlers...)
			s.newHandlers = s.newHandlers[:0]
			// handler to be append
		}
		raw := msg.raw
		src := msg.source.String()
		for i, handler := range s.handlers {
			log.Println("handler ", i, ", raw ", string(raw))
			c := handler(src, raw)
			raw = raw[c:]
			if len(raw) == 0 {
				break
			}
		}
		if len(raw) != 0 {
			log.Println("Some message from ", src, " was dropped ", string(raw))
		}

	}
}

func (s *P2PClient) listen() {
	ch := make(chan Msg, 16)
	go s.handlerThread(ch)
	for { // listening thread
		var err error
		var c int

		msg := Msg{raw: make([]byte, 512)}
		c, msg.source, err = s.conn.ReadFromUDP(msg.raw)
		if err != nil {
			log.Println("read error: ", err)
			continue
		}
		log.Println("c: ", c)
		msg.raw = msg.raw[:c]
		ch <- msg
	}
}

func (s *P2PClient) AddPeer(c *netutils.Client) *PeerIO {
	// TODO lock peerList
	s.peerList = append(s.peerList, *c)

	peer := &PeerIO{
		client: c,
		conn:   s.conn,
	}
	s.ios[c.Public] = peer

	// TODO sending an ack
	_, err := peer.Write([]byte("Joined\n"))
	if err != nil {
		log.Println("Error writing peer ack ", err)
	}

	log.Println("Peer added ", c.Public)
	return peer
}

func (s P2PClient) Close() {
	s.conn.Close()
}
