package main

import (
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	netutils "github.com/cyk451/hole-punching/src/net-utils"
)

const CLIENT_PORT = "11711"
const HOST = "10.128.113.10:" + CLIENT_PORT

// var peerUdps = []net.Conn{}

// var peerChannel chan []byte
var peerList []netutils.Client
var self = netutils.Client{}

//func shellHandler(udp) {
//	for {
//		fmt.Print("$ ")
//		cmdString, err := reader.ReadString('\n')
//		if err != nil {
//			fmt.Fprintln(os.Stderr, err)
//		}
//		// cmdString = strings.TrimSuffix(cmdString, "\n")
//
//		sendMessage(cmdString)
//		/*
//			cmd := exec.Command(cmdString)
//			cmd.Stderr = os.Stderr
//			cmd.Stdout = os.Stdout
//			err = cmd.Run()
//			if err != nil {
//				fmt.Fprintln(os.Stderr, err)
//			}
//		*/
//	}
//
//}
//
//func sendMessage(msg string) {
//
//}
//

type P2PConnection struct {
	conn    *net.UDPConn
	decoder *gob.Decoder
	hostUDP *net.UDPAddr
	hostIo  HostIO
	peerIo  PeerIO
	localIP string
}

type HostIO struct {
	conn *net.UDPConn
}

func (s P2PConnection) WriteHost(p []byte) (l int, e error) {
	return s.conn.WriteToUDP(p, s.hostUDP)
}

func (s P2PConnection) ReadHost(p []byte) (l int, e error) {
	return s.hostIo.Read(p)
}

// actually read from master
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

type PeerIO struct {
	conn *net.UDPConn
}

func (s PeerIO) Read(p []byte) (int, error) {
	return 0, nil
}

func (s PeerIO) Write(p []byte) (int, error) {
	for i := range peerList {
		la, err := net.ResolveUDPAddr("udp", peerList[i].Public)
		if err != nil {
			fmt.Println("udp resolving error ", err)
		}

		_, err = s.conn.WriteToUDP(p, la)
		if err != nil {
			fmt.Println("Error writing to peer ", peerList[i].Public, ", ", err)
			return 0, err
		}
		fmt.Println("bytes wrote to ", peerList[i].Public)
	}
	return len(p), nil
}

func dialToPeers() *PeerIO {
	/*
		for {
			select {
			case msg <- peerChannel:
				fmt.Println("received ", msg)
			}
		}
	*/
	return nil
}

//
func establishPeerConnection(conn *netutils.Client) {
}

//
//func isNewPeer() bool {
//	return true
//}
//

func NewP2PConnection(host string) *P2PConnection {
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

	c := &P2PConnection{}
	c.hostUDP = hostUDP
	c.conn = conn
	c.localIP = localIP
	c.hostIo = HostIO{c.conn}
	c.peerIo = PeerIO{c.conn}
	c.decoder = gob.NewDecoder(c.hostIo)

	return c
}

func (s P2PConnection) ReadSerialized(obj interface{}) (err error) {
	for retries := 10; retries > 0; retries-- {
		err = s.decoder.Decode(obj)
		// somehow we need to do a couple retries
		if err == nil {
			break
		}
	}
	return

}

func dialToServer(initReady chan *P2PConnection) {
	p2p := NewP2PConnection(HOST)

	what := "register " + p2p.localIP

	// send a registeration information to host
	n, err := p2p.WriteHost([]byte(what))
	if err != nil {
		fmt.Println(err, ", n: ", n)
		return // err
	}

	err = p2p.ReadSerialized(&self)
	if err != nil {
		fmt.Println("reading client ", err)
		return
	}
	fmt.Printf("got assigned id %d\n", self.Id)

	n, err = p2p.WriteHost([]byte("list "))
	if err != nil {
		fmt.Println(err, ", n: ", n)
		return // err
	}

	var hdr netutils.Header
	err = p2p.ReadSerialized(&hdr)
	if err != nil {
		fmt.Println("reading client ", err)
		return
	}

	fmt.Println("got ", hdr.Num, " peers")

	peerList = make([]netutils.Client, 0, hdr.Num)

	for i := 0; i < hdr.Num; i++ {
		var c netutils.Client
		err = p2p.ReadSerialized(&c)
		peerList = append(peerList, c)
		fmt.Printf("I got client %+v\n", c)
	}

	initReady <- p2p

	for {
		msg := make([]byte, 512)
		n, addr, err := p2p.conn.ReadFromUDP(msg)
		if err != nil {
			log.Println("read error: ", err)
			continue
		}
		// TODO if the sender absents in peer list ask host for a update

		log.Println("From ", addr.IP.String(), " Message: ", string(msg[:n]))
	}
}

func main() {

	var p2p *P2PConnection
	initReady := make(chan *P2PConnection)
	go dialToServer(initReady)
	p2p = <-initReady

	tick := time.Tick(5000 * time.Millisecond)
	for {
		select {
		case <-tick:
			p2p.peerIo.Write([]byte("Hello peers"))
		}
	}

	// go dialToPeers()
	// shellHandler()
}
