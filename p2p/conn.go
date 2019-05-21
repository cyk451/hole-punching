package p2p

import (
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"github.com/cyk451/hole-punching/proto_models"
	"github.com/golang/protobuf/proto"
)

/*
  The connection is considered timeout if the target doesn't respond to a
  handshake in this time.
*/
const PeerAckTimeoutSec = 1

const AckString = "Ackme"

type Client = proto_models.Client

/*
  This is a token asking others to reply.
*/
var AckBytes = []byte(AckString)

/*
  The maximum bytes can be read in one read operation.
*/
const MaxByteRead = 512

/*
  A handler is a reader callback. Once registered, it would be invoked whenever
  the incoming udp data presents.

  Keep handler quick and simple. If there is any chance the handler could
  blocks, it's callers' responsiblility to fork.
*/
type Handler func(source string, msg []byte) (processed int)

// TODO: should we export these structs?
type LockedUDPConn struct {
	*net.UDPConn
	mutex sync.Mutex
}

type Writer interface {
	Write([]byte) (int, error)
	WriteSerialized(obj proto.Message) error
	GetReader() Handler
	SetReader(Handler)

	updateReader()
}

type p2pStatus int

const (
	NotInit p2pStatus = 0
	Running p2pStatus = 1
	Stopped p2pStatus = 2
)

type Conn struct {
	/*
		The ip address this connection is listening to.
	*/
	LocalIP string
	/*
		A code indicating current status of connection.
	*/
	Status p2pStatus

	*LockedUDPConn
	hostUDP     *net.UDPAddr // could be nil
	mapLock     sync.Mutex
	ios         map[string]Writer
	handlers    []Handler
	newHandlers []Handler
}

/*
 Always use this to initialize an object; it performs some essential
 initializations,
*/
func NewConn() *Conn {
	c := &Conn{
		Status: Stopped,
		ios:    make(map[string]Writer),
	}
	return c
}

/*
 This broadcast serialized object to every connected peer. Note it won't check
 any ack at all.
*/
func (s *Conn) WriteToPeers(obj proto.Message) (err error) {

	s.mapLock.Lock()
	for _, t := range s.ios {
		_, ok := t.(*PeerIO)
		if !ok {
			continue
		}
		e := t.WriteSerialized(obj)
		if e != nil {
			err = e
		}
	}
	s.mapLock.Unlock()
	return err
}

func (s *Conn) AddSourcedHandler(expecting string, pureHandler func([]byte) int) {
	s.AddHandler(
		func(source string, raw []byte) int {
			if source != expecting {
				return 0
			}
			return pureHandler(raw)
		},
	)
}

/*
 It's important to realize that adding new handlers in a handler is possible,
 but the added handlers wouldn't be in play until the next iteration.
*/
func (s *Conn) AddHandler(h Handler) {
	s.newHandlers = append(s.newHandlers, h)
}

/*
 This sends a register command to host; put itself into the peer list.
*/
func (s *Conn) Register() error {
	if s.hostUDP == nil {
		return errors.New("This connection doesn't have a host")
	}
	s.mapLock.Lock()
	host, ok := s.ios[s.hostUDP.String()]
	s.mapLock.Unlock()
	if !ok {
		return errors.New("Host not connected")
	}

	what := "register " + s.LocalIP
	_, err := host.Write([]byte(what))
	if err != nil {
		return err
	}
	return nil
}

func findLocalIP(hostUDP *net.UDPAddr) (string, error) {
	conn, err := net.DialUDP("udp", nil, hostUDP)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	return conn.LocalAddr().String(), nil
}

/*
 It's possible to listen to incoming connections without connecting to a p2p
 host.
*/
func (s *Conn) ConnectHost(hostip string) (*HostIO, error) {
	hostUDP, err := net.ResolveUDPAddr("udp", hostip)
	if err != nil {
		return nil, err
	}

	LocalIP, err := findLocalIP(hostUDP)
	if err != nil {
		return nil, err
	}

	// ok now we need an unconnected udp connection listening on the port we
	// just used.
	s.hostUDP = hostUDP

	s.ListenAsync(LocalIP)

	host := &HostIO{NewIOFromIP(s, hostip)}
	s.mapLock.Lock()
	s.ios[hostip] = host
	s.mapLock.Unlock()

	return host, nil
}

func (s *Conn) ListenAsync(ip string) error {
	err := s.prepareListen(ip)
	if err != nil {
		return err
	}
	go s.listenThread()

	return nil
}

/*
 This make udp listen on given ip and send received message to handlers. This
 function blocks.
*/
func (s *Conn) Listen(ip string) error {
	err := s.prepareListen(ip)
	if err != nil {
		return err
	}
	s.listenThread()
	return nil
}

func (s *Conn) prepareListen(ip string) (err error) {
	if s.Status == Running {
		err = errors.New("p2p is already listening")
		return
	}

	// listening peer connection
	udpAddr, err := net.ResolveUDPAddr("udp", ip)
	if err != nil {
		return
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return
	}

	s.LockedUDPConn = &LockedUDPConn{UDPConn: conn}
	s.LocalIP = ip
	s.Status = Running

	// This was a common global handler
	s.AddHandler(ackHandlerBindP2PConn(s))
	return
}

/*
func (s *Conn) listenReadySync(listenIP string, ready chan<- error) (err error) {

	s.listenThread()
	return nil

}
*/

type messageSet struct {
	source *net.UDPAddr
	raw    []byte
}

func (s *Conn) updateHandlers() {
	if len(s.newHandlers) != 0 {
		s.handlers = append(s.handlers, s.newHandlers...)
		s.newHandlers = s.newHandlers[:0]
	}
}

/* handle real jobs in this thread */
func (s *Conn) handlerThread(ch chan messageSet) {
	for {
		msg := <-ch

		raw := msg.raw
		src := msg.source.String()

		log.Println("src: ", src, ", raw: '", string(raw), "'")
		s.mapLock.Lock()
		io, ok := s.ios[src]
		s.mapLock.Unlock()
		if ok {
			io.updateReader()

			handler := io.GetReader()
			if handler != nil {
				c := handler(src, raw)
				raw = raw[c:]
			}
		}

		if len(raw) == 0 {
			continue
		}

		s.updateHandlers()
		/* global handlers */
		for _, handler := range s.handlers {
			c := handler(src, raw)
			raw = raw[c:]
			// no message to be processed
			if len(raw) == 0 {
				break
			}
		}

		if len(raw) != 0 {
			log.Println("Some message from ", src, " was dropped ", string(raw))
		}
	}
}

func (s *Conn) listenThread() {
	ch := make(chan messageSet, 16)
	go s.handlerThread(ch)

	for { // listening thread
		in := make([]byte, MaxByteRead)
		c, s, err := s.ReadFromUDP(in)
		if err != nil {
			// log.Println("read error: ", err)
			continue
		}
		ch <- messageSet{
			source: s,
			raw:    in[:c],
		}
	}
}

/* if nil is returned, either this ip not exist, or it's not a peer */
func (s *Conn) GetPeerByIP(ip string) *PeerIO {
	s.mapLock.Lock()
	r, ok := s.ios[ip]
	s.mapLock.Unlock()
	if ok {
		p, ok := r.(*PeerIO)
		if ok {
			return p
		}
	}
	return nil
}

func (s *Conn) GetPeer(c *Client) *PeerIO {
	p := s.GetPeerByIP(c.Public)
	if p != nil {
		return p
	}
	if c.Private != "" {
		p = s.GetPeerByIP(c.Private)
		if p != nil {
			return p
		}
	}
	return nil
}

func (s *Conn) AddPeer(c *Client) *PeerIO {

	if p := s.GetPeer(c); p != nil {
		return p
	}

	faster := make(chan string)

	var (
		peer        *PeerIO
		privatePeer *PeerIO
		publicPeer  *PeerIO
	)
	publicPeer = &PeerIO{NewIOFromIP(s, c.Public), c}
	s.mapLock.Lock()
	s.ios[c.Public] = publicPeer
	s.mapLock.Unlock()
	udpHandShakeSync(publicPeer, faster)

	if c.Private != "" {
		privatePeer = &PeerIO{NewIOFromIP(s, c.Private), c}
		s.mapLock.Lock()
		s.ios[c.Private] = privatePeer
		s.mapLock.Unlock()
		udpHandShakeSync(privatePeer, faster)
	}

	retries := 5
	for retries >= 0 {
		t := time.NewTimer(PeerAckTimeoutSec * time.Second)
		var winner string
		select {
		case winner = <-faster:
			if winner == c.Public {
				peer = publicPeer
				s.mapLock.Lock()
				delete(s.ios, c.Private)
				s.mapLock.Unlock()
			} else {
				peer = privatePeer
				s.mapLock.Lock()
				delete(s.ios, c.Public)
				s.mapLock.Unlock()
			}

			return peer
		case <-t.C:
			log.Println("Retry ack ", c)
			publicPeer.Write(AckBytes)
			if privatePeer != nil {
				privatePeer.Write(AckBytes)
			}
			retries -= 1
		}
	}
	return nil

}

func (s Conn) Close() {
	s.LockedUDPConn.Close()
	s.Status = Stopped
}

/*
func (s conn)markClientAlive(ip string) {
	for i := range s.Clients {
	}
}
*/

func ackHandlerBindP2PConn(c *Conn) Handler {
	return func(src string, msg []byte) int {
		if string(msg) == AckString {
			// markClientAlive(src)
			udp, err := net.ResolveUDPAddr("udp", src)
			if err != nil {
				// return 0, err
				return 0
			}

			log.Println(c.LockedUDPConn)
			c.LockedUDPConn.mutex.Lock()
			defer c.LockedUDPConn.mutex.Unlock()
			_, err = c.LockedUDPConn.WriteToUDP([]byte("Received"), udp)
			if err != nil {
				log.Println("udp write: ", err)
			}
			log.Println("src: ", src, ", acked")
			return len(msg)
		}
		return 0
	}
}

func udpHandShakeSync(w Writer, readBack chan<- string) {
	w.SetReader(func(src string, msg []byte) int {
		if string(msg) == "Received" {
			// markClientAlive(src)
			log.Println("src: ", src, ", Received")
			readBack <- src
			return len(msg)
		}
		return 0
	})
	w.Write(AckBytes)
}

func writeToIP(conn *LockedUDPConn, ip string, p []byte) (int, error) {
	return 0, nil
}
