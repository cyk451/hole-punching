package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	netutils "github.com/cyk451/hole-punching/src/net-utils"
)

const HOST_PORT = "11711"

// const HOST = "10.128.113.10:" + HOST_PORT

const HOST = "104.155.225.82:" + HOST_PORT
const CURSOR = "$ "

func chatShellHandler(p2p *P2PClient) {
	// var err error

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print(CURSOR)
		what, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println("bye")
				return
			}
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		go p2p.WriteToPeers([]byte(what))
	}
}

type PeerIO struct {
	client *netutils.Client
	conn   *LockedUDPConn
	io.Reader
	readPipeWriter io.Writer
	udp            *net.UDPAddr
	// reuse Read()
	// conn           *P2PClient
}

func (s *PeerIO) writeToReadPipe(bytes []byte) (int, error) {
	return s.readPipeWriter.Write(bytes)
}

// actually read from master
/*
func (s HostIO) Read(p []byte) (l int, e error) {
	l, addr, e := s.conn.ReadFromUDP(p)
	// debug.PrintStack()
	source := addr.String()
	if e != nil || source != HOST {
		if e == nil {
			e = errors.New("Expecting data from host... got from " + source)
		}
		fmt.Println("Read error ", e)
		return
	}
	// fmt.Println("bytes read: ", p[:l])
	return
}

func (hs HostIO) Handle(msg []byte) error {
	// keep reading peer data
	var c netutils.Client

	err := gob.NewDecoder(bytes.NewBuffer(msg)).Decode(c)
	if err != nil {
		return err
	}

	peerList = append(peerList, c)
	return nil
}
*/

func (s *PeerIO) Write(p []byte) (c int, err error) {
	// TODO: try private and public
	if s.udp == nil {
		public := s.client.Public
		s.udp, err = net.ResolveUDPAddr("udp", public)
		if err != nil {
			log.Println("udp resolving error ", err)
		}
	}

	s.conn.mutex.Lock()
	c, err = s.conn.WriteToUDP(p, s.udp)
	s.conn.mutex.Unlock()

	if err != nil {
		log.Println("Error writing to peer ", s.udp, ", ", err)
		return
	}
	log.Println(c, " bytes wrote to ", s.udp)
	return
}

func peerReadHandler(peer *PeerIO) {
	for {
		msg := make([]byte, 512)
		l, err := peer.Read(msg)
		if err != nil {
			if err == io.EOF {
				continue
			}
			log.Println("reading client: ", err)
			break
		}
		fmt.Print("\r" + string(msg[:l]) + CURSOR)
	}
	log.Println("Read thread for peer", peer.client.Public, " died")
}

func listen(p2p *P2PClient, host *HostIO) {
	for {
		var c netutils.Client
		err := host.ReadSerialized(&c)
		if err != nil {
			log.Println("ReadSerialized error: ", err)
			continue
		}

		log.Printf("Adding peer %+v\n", c)
		peer := p2p.AddPeer(&c)
		go peerReadHandler(peer)
	}
}

func signalExit(host *HostIO) {
	log.Println("Close")

	host.Write([]byte("close"))

}

func main() {
	p2p := newP2PClient(HOST)

	// send a registeration information to host
	host, err := p2p.ConnectHost()
	if err != nil {
		log.Fatal("Error dial p2p ", err)
		return
	}

	go p2p.Listen()

	go listen(p2p, host)

	fmt.Println("* Client runing on ", p2p.localIP)

	chatShellHandler(p2p)

	signalExit(host)
}
