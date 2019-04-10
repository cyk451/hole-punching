package main

import (
	"encoding/gob"
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

type PeerStream struct {
}

func (s PeerStream) Read(p []byte) (int, error) {
	return 0, nil
}

func (s PeerStream) Write(p []byte) (int, error) {
	for i := range peerList {
		la, err := net.ResolveUDPAddr("udp", peerList[i].Public)
		if err != nil {
			fmt.Println("udp resolving error ", err)
		}

		conn, err := net.DialUDP("udp", nil, la)

		// conn, err := net.DialUDP("udp", peerList[i].public)
		if err != nil {
			fmt.Println("udp dial error ", err)
			return 0, err
		}

		defer conn.Close()
		_, err = conn.Write(p)
		if err != nil {
			fmt.Println("Error writing to peer ", peerList[i].Public, ", ", err)
			return 0, err
		}
		fmt.Println("bytes wrote to ", peerList[i].Public)
	}
	return len(p), nil
}

func dialToPeers() *PeerStream {
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

type ServerReader struct {
	conn    *net.UDPConn
	decoder *gob.Decoder
}

func NewServerReader(conn *net.UDPConn) *ServerReader {
	c := &ServerReader{}
	c.conn = conn
	c.decoder = gob.NewDecoder(c)
	return c
}

func (s ServerReader) Read(p []byte) (l int, e error) {
	l, e = s.conn.Read(p)
	// debug.PrintStack()
	if e != nil {
		fmt.Println("Read error ", e)
		return
	}
	// fmt.Println("bytes read: ", p[:l])
	return
}

func (s ServerReader) ReadSerialized(obj interface{}) (err error) {
	for retries := 10; retries > 0; retries-- {
		err = s.decoder.Decode(obj)
		// somehow we need to do a couple retries
		if err == nil {
			break
		}
	}
	return

}

func dialToServer(initReady chan struct{}) {

	la, err := net.ResolveUDPAddr("udp", HOST)
	if err != nil {
		fmt.Println("udp resolving error ", err)
	}

	conn, err := net.DialUDP("udp", nil, la)
	if err != nil {
		fmt.Println("Dial, ", err)
		return // err
	}

	sr := NewServerReader(conn)

	what := "register " + conn.LocalAddr().String()

	fmt.Println("sending ", what)

	// send a register information to host
	n, err := conn.Write([]byte(what))
	if err != nil || n != len(what) {
		fmt.Println(err, ". n & len ", n, len(what))
		return // err
	}

	err = sr.ReadSerialized(&self)
	if err != nil {
		fmt.Println("reading client ", err)
		return
	}
	fmt.Printf("got assigned id %d\n", self.Id)

	n, err = conn.Write([]byte("list "))
	if err != nil {
		fmt.Println(err, ". n & len ", n, len(what))
		return // err
	}

	var hdr netutils.Header
	err = sr.ReadSerialized(&hdr)
	if err != nil {
		fmt.Println("reading client ", err)
		return
	}

	fmt.Println("got ", hdr.Num, " peers")

	peerList = make([]netutils.Client, 0, hdr.Num)

	for i := 0; i < hdr.Num; i++ {
		var c netutils.Client
		err = sr.ReadSerialized(&c)
		peerList = append(peerList, c)
		fmt.Printf("I got client %+v\n", c)
	}

	conn.Close()

	initReady <- struct{}{}

	// listening peer connection
	udpAddr, err := net.ResolveUDPAddr("udp4", self.Private)
	if err != nil {
		log.Fatal(err)
	}

	ln, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal(err)
	}

	for {
		msg := make([]byte, 512)
		n, addr, err := ln.ReadFromUDP(msg)
		if err != nil {
			log.Println("read error: ", err)
			continue
		}
		// TODO if the sender absents in peer list ask host for a update

		log.Println("From ", addr.IP.String(), " Message: ", string(msg[:n]))
	}
}

func main() {

	initReady := make(chan struct{})
	go dialToServer(initReady)
	<-initReady

	tick := time.Tick(5000 * time.Millisecond)
	for {
		select {
		case <-tick:
			PeerStream{}.Write([]byte("Hello peers"))
		}
	}

	// go dialToPeers()
	// shellHandler()
}
