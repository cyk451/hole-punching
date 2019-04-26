package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"flag"
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
		if what[0] == '\n' {
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
	lastResponse   uint64
	// reuse Read()
	// conn           *P2PClient
}

func (s *PeerIO) writeToReadPipe(bytes []byte) (int, error) {
	return s.readPipeWriter.Write(bytes)
}

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
	// log.Println(c, " bytes wrote to ", s.udp)
	return
}

func writeMessage(what string) {
	fmt.Print("\r" + what + CURSOR)
}

// A handler takes source and raw bytes. It returns a int to indicate how much
// bytes it processed, the rest of the texts are then passed to the next
// handler in the chain.
func peerReadHandler(raw []byte) int {
	writeMessage(string(raw))
	return len(raw)
}

func signalExit(host *HostIO) {
	log.Println("Close")

	host.Write([]byte("close"))

}

/* arguments */
var (
	h      bool
	logfn  string
	hostip string
)

func parseArgs() {

	flag.BoolVar(&h, "h", false, "this help")
	flag.StringVar(&logfn, "l", "STDERR", "a filename to which logs are printed. upon open errors, write to stderr")
	flag.StringVar(&hostip, "H", HOST, "a ip address of hole punching name server to connect to")
}

func openLogFile() *os.File {
	if logfn == "STDERR" {
		return nil
	}
	logf, err := os.OpenFile(logfn, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.Println("Failed opening log file, logs to stderr: ", err)
	} else {
		log.Println("logs redirected to ", logfn)
		log.SetOutput(logf)
	}
	return logf
}

func main() {
	parseArgs()
	flag.Parse()
	log.Println("h ", h)
	log.Println("logfn ", logfn)
	log.Println("hostip ", hostip)
	if h {
		flag.Usage()
		return
	}

	if logf := openLogFile(); logf != nil {
		defer logf.Close()
	}

	p2p := newP2PClient(hostip)

	host, err := p2p.ConnectHost()
	if err != nil {
		log.Fatal("Error dial p2p ", err)
		return
	}

	p2p.AddSourcedHandler(
		host.udp.String(),
		func(raw []byte) (bRead int) {
			var c netutils.Client

			rdr := bytes.NewReader(raw)
			had := rdr.Len()
			err := gob.NewDecoder(rdr).Decode(&c)
			if err != nil {
				log.Println("not accespt: reason ", err)
				return
			}

			peer := p2p.AddPeer(&c)
			p2p.AddSourcedHandler(
				peer.udp.String(),
				peerReadHandler,
			)
			bRead = had - rdr.Len()
			return
		},
	)

	// send a registeration information to host
	err = p2p.Register()
	if err != nil {
		log.Println("Register error ", err)
	}

	// go listen(p2p, host)

	fmt.Println("* Client runing on ", p2p.localIP)

	chatShellHandler(p2p)

	signalExit(host)
}
