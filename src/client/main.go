package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/cyk451/hole-punching/src/hpclient"
	protobuf "github.com/gogo/protobuf/types"
	"github.com/golang/protobuf/proto"
)

const HOST_PORT = "11711"

// const HOST = "10.128.113.10:" + HOST_PORT

const HOST = "104.155.225.82:" + HOST_PORT
const CURSOR = "$ "

func asMessage(w string) *hpclient.RawMessage {
	now := time.Now()
	return &hpclient.RawMessage{
		Msg: w,
		Time: &protobuf.Timestamp{
			Seconds: now.Unix(),
			Nanos:   int32(now.UnixNano()),
		},
	}

}

func chatShellHandler(p2p *P2PClient) {
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
		go p2p.WriteToPeers(asMessage(string(what)))
	}
}

func printMsg(msg *hpclient.RawMessage) {
	fmt.Print("\r" +
		time.Unix(msg.Time.Seconds, int64(msg.Time.Nanos)).Format("03:04:05") +
		": " + msg.Msg + CURSOR)
}

// A handler takes source and raw bytes. It returns a int to indicate how much
// bytes it processed, the rest of the texts are then passed to the next
// handler in the chain.
func peerReadHandler(raw []byte) int {

	msg := &hpclient.RawMessage{}
	err := proto.Unmarshal(raw, msg)
	if err != nil {
		log.Println("not accespt: reason ", err)
		return 0
	}
	printMsg(msg)

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
	flag.StringVar(&hostip, "H", HOST, "an ip address on which a hole punching name server is running")

	flag.Parse()
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

	if h {
		flag.Usage()
		return
	}

	if logf := openLogFile(); logf != nil {
		defer logf.Close()
	}

	p2p := &P2PClient{}

	host, err := p2p.ConnectHost(hostip)
	if err != nil {
		log.Fatal("Error dial p2p ", err)
		return
	}
	defer signalExit(host)

	p2p.AddSourcedHandler(
		host.udp.String(),
		func(raw []byte) (bRead int) {
			c := &hpclient.Client{}

			err := proto.Unmarshal(raw, c)
			if err != nil {
				log.Println("not accespt: reason ", err)
				return
			}

			peer := p2p.AddPeer(c)
			p2p.AddSourcedHandler(
				peer.client.Public,
				peerReadHandler,
			)

			err = peer.WriteSerialized(asMessage("Joined\n"))
			if err != nil {
				log.Println("Error writing peer ack ", err)
			}

			bRead = len(raw)
			return
		},
	)

	// send a registeration information to host
	err = p2p.Register()
	if err != nil {
		log.Println("Register error ", err)
	}

	fmt.Println("* Local IP: ", p2p.localIP)

	chatShellHandler(p2p)
}
