package main

import (
	"encoding/gob"
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

type Message struct {
	from    *net.UDPAddr
	content []byte
}

type HostIO struct {
	conn *net.UDPConn
	// buf     bytes.Buffer
	decoder *gob.Decoder
	// signal  chan int
	// size    int
	// io.Pipe
	// buf io.Reader
}
type PeerIO struct {
	conn *net.UDPConn
	// bytes.Buffer
	// decoder *gob.Decoder
}

func (s HostIO) ReadSerialized(obj interface{}) (err error) {
	for retries := 10; retries > 0; retries-- {
		err = s.decoder.Decode(obj)
		// somehow we need to do a couple retries
		if err == nil {
			return
		}
	}
	return
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

func (s PeerIO) Read(p []byte) (int, error) {
	return 0, nil
}

func (s PeerIO) Write(p []byte) (int, error) {
	for i := range peerList {
		la, err := net.ResolveUDPAddr("udp", peerList[i].Public)
		if err != nil {
			log.Println("udp resolving error ", err)
		}

		_, err = s.conn.WriteToUDP(p, la)
		if err != nil {
			log.Println("Error writing to peer ", peerList[i].Public, ", ", err)
			return 0, err
		}
		log.Println("bytes wrote to ", peerList[i].Public)
	}
	return len(p), nil
}

func dialToPeers() *PeerIO {
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

func dialToServer(initReady chan *P2PClient) {
	p2p := newP2PConnection(HOST)

	// send a registeration information to host

	host, err := p2p.ConnectHost()
	if err != nil {
		log.Fatal("Error ", err)
		return
	}

	/*
		err = p2p.ReadSerialized(&self)
		if err != nil {
			fmt.Println("reading client ", err)
			return
		}
		fmt.Printf("got assigned id %d\n", self.Id)
	*/

	/*
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
	*/

	initReady <- p2p

	go p2p.Listen()

	for {
		var c netutils.Client
		err := host.ReadSerialized(&c)
		if err != nil {
			// log.Println("ReadSerialized error: ", err)
			continue
		}

		log.Printf("Adding peer %+v\n", c)
		peerList = append(peerList, c)
	}
}

func main() {

	var p2p *P2PClient
	initReady := make(chan *P2PClient)
	go dialToServer(initReady)
	p2p = <-initReady

	log.Println("Client on ", p2p.localIP)

	tick := time.Tick(5000 * time.Millisecond)
	for {
		select {
		case <-tick:
			p2p.peerIo.Write([]byte("Hello peers"))
		}
	}

	// go dialToPeers()
	shellHandler(p2p)
}
