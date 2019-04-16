package main

import (
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	netutils "github.com/cyk451/hole-punching/src/net-utils"
)

type p2pHandler interface {
	Handle(msg []byte) error
}

type p2pIO interface {
	Read([]byte) (int, error)

	Write([]byte) (int, error)

	writeToReadPipe([]byte) (int, error)
}

type LockedUDPConn struct {
	*net.UDPConn
	mutex sync.Mutex
}

type HostIO struct {
	conn           *LockedUDPConn
	udp            *net.UDPAddr
	decoder        *gob.Decoder
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
		err = s.decoder.Decode(obj)
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
	ios          map[string]p2pIO
	self         netutils.Client
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
	c.ios = make(map[string]p2pIO)

	return c
}

func (s *P2PClient) ConnectHost() (*HostIO, error) {
	r, w := io.Pipe()

	host := &HostIO{
		decoder:        gob.NewDecoder(r),
		Reader:         r,
		readPipeWriter: w,
		udp:            s.hostUDP,
		conn:           s.conn,
	}
	s.ios[s.hostUDP.String()] = host

	what := "register " + s.localIP
	n, err := host.Write([]byte(what))
	if err != nil {
		fmt.Println(err, ", n: ", n)
		return nil, err
	}

	return host, nil
}

func (s *P2PClient) handle(msg []byte, source string) {
	h, found := s.ios[source]
	if !found {
		log.Println("msg from ", source, " has no handler: ", msg)
		return
	}

	log.Println("Some messages from ", source, string(msg))

	_, err := h.writeToReadPipe(msg)
	if err != nil {
		log.Println("error handling msg from ", source, ": ", err)
	}
}

func (s *P2PClient) Listen() {
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

func (s *P2PClient) AddPeer(c *netutils.Client) *PeerIO {
	// TODO lock peerList
	// i := len(s.peerList)
	s.peerList = append(s.peerList, *c)
	r, w := io.Pipe()

	peer := &PeerIO{
		client:         c,
		conn:           s.conn,
		Reader:         r,
		readPipeWriter: w,
	}
	s.ios[c.Public] = peer
	log.Println("Peer added ", c.Public)
	return peer
}

func (s P2PClient) Close() {
}
