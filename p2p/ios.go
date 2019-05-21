package p2p

import (
	"log"
	"net"

	"github.com/cyk451/hole-punching/proto_models"
	"github.com/golang/protobuf/proto"
)

// common functions shared by peer & host
type IOBase struct {
	conn *LockedUDPConn
	udp  *net.UDPAddr

	readHandler Handler
	newHandler  Handler
}

func NewIOFromIP(s *Conn, ip string) *IOBase {
	udp, err := net.ResolveUDPAddr("udp", ip)
	if err != nil {
		log.Println("resolve error ", ip)
		return nil
	}
	return &IOBase{
		conn: s.LockedUDPConn,
		udp:  udp,
	}
}

func (s *IOBase) SetReader(h Handler) {
	s.newHandler = h
}
func (s *IOBase) GetReader() Handler {
	return s.readHandler
}

func (s *IOBase) updateReader() {
	if s.newHandler != nil {
		s.readHandler = s.newHandler
		s.newHandler = nil
	}
}

func (s *IOBase) Write(p []byte) (c int, err error) {
	s.conn.mutex.Lock()
	defer s.conn.mutex.Unlock()
	return s.conn.WriteToUDP(p, s.udp)
}

func (s *IOBase) IP() string {
	return s.udp.String()
}

func (s *IOBase) WriteSerialized(obj proto.Message) error {
	out, err := proto.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = s.Write(out)
	return err
}

type PeerIO struct {
	*IOBase
	Client *proto_models.Client
}

type HostIO struct {
	*IOBase
}
