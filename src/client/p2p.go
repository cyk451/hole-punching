package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/cyk451/hole-punching/src/hpclient"
	"github.com/golang/protobuf/proto"
)

// TODO make this p2pconnection a standalone package
type P2PHandler func(source string, msg []byte) (processed int)

type LockedUDPConn struct {
	*net.UDPConn
	mutex sync.Mutex
}

type P2PWriter interface {
	Write([]byte) (int, error)
	WriteSerialized(obj proto.Message) error
}

type HostIO struct {
	conn *LockedUDPConn
	udp  *net.UDPAddr
}

var _ P2PWriter = &HostIO{}

func (s *HostIO) Write(bytes []byte) (i int, e error) {
	s.conn.mutex.Lock()
	defer s.conn.mutex.Unlock()
	return s.conn.WriteToUDP(bytes, s.udp)
}

func (s *HostIO) WriteSerialized(obj proto.Message) error {
	out, err := proto.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = s.Write(out)
	return err
}

type PeerIO struct {
	client *hpclient.Client
	conn   *LockedUDPConn
	udp    *net.UDPAddr
}

var _ P2PWriter = &PeerIO{}

func (s *PeerIO) Write(p []byte) (c int, err error) {
	// TODO: try private and public
	if s.udp == nil {
		s.udp, err = net.ResolveUDPAddr("udp", s.client.Public)
		if err != nil {
			log.Println("udp resolving error ", err)
			return 0, err
		}
	}

	s.conn.mutex.Lock()
	c, err = s.conn.WriteToUDP(p, s.udp)
	s.conn.mutex.Unlock()

	if err != nil {
		log.Println("Error writing to peer ", s.udp, ", ", err)
		return
	}
	return
}

func (s *PeerIO) WriteSerialized(obj proto.Message) error {
	out, err := proto.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = s.Write(out)
	return err
}

type P2PStatus int

const (
	NotInit P2PStatus = 0
	Running P2PStatus = 1
	Stopped P2PStatus = 2
)

type P2PClient struct {
	conn        *LockedUDPConn
	hostUDP     *net.UDPAddr
	localIP     string
	peerList    []hpclient.Client
	ios         map[string]P2PWriter
	handlers    []P2PHandler
	newHandlers []P2PHandler

	Status P2PStatus
}

func (p2p *P2PClient) WriteToPeers(obj proto.Message) (err error) {
	for _, t := range p2p.ios {
		_, ok := t.(*PeerIO)
		if !ok {
			continue
		}
		err = t.WriteSerialized(obj)
		if err != nil {
			log.Println("Write to peers: ", err)
		}
	}
	return err
}

func findLocalIP(hostUDP *net.UDPAddr) (string, error) {

	// this give you a connected udp.
	conn, err := net.DialUDP("udp", nil, hostUDP)
	if err != nil {
		fmt.Println("Dial, ", err)
		return "", err // err
	}
	defer conn.Close()

	return conn.LocalAddr().String(), nil
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
	if s.hostUDP == nil {
		return errors.New("This connection doesn't have a host")
	}
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

/*
This function must be the very first function called on p2pclient object, it
includes some vital initializations.
*/
func (s *P2PClient) ConnectHost(hostip string) (*HostIO, error) {
	s.ios = make(map[string]P2PWriter)

	hostUDP, err := net.ResolveUDPAddr("udp", hostip)
	if err != nil {
		fmt.Println("udp resolving error ", err)
		return nil, err
	}

	localIP, err := findLocalIP(hostUDP)
	if err != nil {
		return nil, err
	}

	// ok now we need an unconnected udp connection listening on the port we
	// just used.
	s.hostUDP = hostUDP

	err = s.Listen(localIP)
	if err != nil {
		return nil, err
	}

	host := &HostIO{
		udp:  hostUDP,
		conn: s.conn,
	}
	s.ios[hostip] = host

	return host, nil
}

func (s *P2PClient) Listen(listenIP string) error {

	if s.Status == Running {
		return errors.New("p2p is already listening")
	}

	// listening peer connection
	udpAddr, err := net.ResolveUDPAddr("udp", listenIP)
	if err != nil {
		log.Fatal(err)
		return err
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal(err)
		return err
	}

	s.conn = &LockedUDPConn{UDPConn: conn}
	s.localIP = listenIP
	s.Status = Running

	go s.listenThread()
	return nil

}

type messageSet struct {
	source *net.UDPAddr
	raw    []byte
}

func (s *P2PClient) updateHandlers() {
	if len(s.newHandlers) != 0 {
		s.handlers = append(s.handlers, s.newHandlers...)
		s.newHandlers = s.newHandlers[:0]
	}
}

func (s *P2PClient) handlerThread(ch chan messageSet) {
	for {
		msg := <-ch

		s.updateHandlers()

		raw := msg.raw
		src := msg.source.String()
		for _, handler := range s.handlers {
			// log.Println("handler ", i, ", raw ", string(raw))
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

func (s *P2PClient) listenThread() {
	ch := make(chan messageSet, 16)
	go s.handlerThread(ch)

	for { // listening thread
		in := make([]byte, 512)
		c, s, err := s.conn.ReadFromUDP(in)
		if err != nil {
			log.Println("read error: ", err)
			continue
		}
		ch <- messageSet{
			source: s,
			raw:    in[:c],
		}
	}
}

func (s *P2PClient) AddPeer(c *hpclient.Client) *PeerIO {
	// TODO lock peerList
	s.peerList = append(s.peerList, *c)

	peer := &PeerIO{
		client: c,
		conn:   s.conn,
	}
	s.ios[c.Public] = peer

	log.Println("Peer added ", c.Public)
	return peer
}

func (s P2PClient) Close() {
	s.conn.Close()
	s.Status = Stopped
}
