package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/cyk451/hole-punching/src/p2p"
	"github.com/cyk451/hole-punching/src/proto_models"
	types "github.com/gogo/protobuf/types"
	"github.com/golang/protobuf/proto"
)

const Version string = "0.1"

/* arguments */
var (
	h        bool
	v        bool
	logfn    string
	hostip   string
	nickname string
)

type Client struct {
	proto_models.Client
	Active string
}

// dummy verbose function
var verbose func(...interface{}) = func(...interface{}) {}

func parseArgs() {

	flag.BoolVar(&h, "h", false,
		"this help")
	flag.BoolVar(&v, "v", false,
		"print verbose logs")
	flag.StringVar(&hostip, "H", HOST,
		"an ip address on which a hole punching name server is running")
	flag.StringVar(&logfn, "l", "STDERR",
		"a filename to which logs are printed. used only if verbose enabled")
	flag.StringVar(&nickname, "n", "anonymous",
		"a name to be showed in chatting")

	flag.Parse()
}

const HOST_PORT = "11711"

// const HOST = "10.128.113.10:" + HOST_PORT

const HOST = "104.155.225.82:" + HOST_PORT
const CURSOR = "< "

func NewMessage(w string) *proto_models.RawMessage {
	now := time.Now()
	return &proto_models.RawMessage{
		Msg: w,
		Time: &types.Timestamp{
			Seconds: now.Unix(),
			Nanos:   int32(now.UnixNano()),
		},
		Who: nickname,
	}

}

func chatShellThread(conn *p2p.Conn) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print(CURSOR)
		what, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				conn.WriteToPeers(NewMessage("Bye\n"))
				return
			}
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		if what[0] == '\n' {
			continue
		}
		msg := NewMessage(string(what))
		printLocalMsg(msg)
		go conn.WriteToPeers(msg)
	}
}

func printMsg(msg *proto_models.RawMessage) {
	fmt.Print("\r" +
		time.Unix(msg.Time.Seconds, int64(msg.Time.Nanos)).Format("03:04:05") +
		"|" + msg.Who + "> " + msg.Msg + CURSOR)
}

func printLocalMsg(msg *proto_models.RawMessage) {
	fmt.Print("\033[A\r" +
		time.Unix(msg.Time.Seconds, int64(msg.Time.Nanos)).Format("03:04:05") +
		"|" + nickname + "< " + msg.Msg)
}

func signalExit(host *p2p.HostIO) {
	verbose("Close")
	host.Write([]byte("close"))
}

func openLogFile() *os.File {
	if logfn == "STDERR" {
		return nil
	}
	logf, err := os.OpenFile(logfn, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		// log.Println("Failed opening log file, logs to stderr: ", err)
		verbose("Failed opening log file, logs to stderr: ", err)
	} else {
		// log.Println("logs redirected to ", logfn)
		verbose("logs redirected to ", logfn)
		log.SetOutput(logf)
	}
	return logf
}

// A handler takes raw bytes. It returns an int to indicate how much bytes it
// processed, the rest of the texts are then passed to the next handler in the
// chain.
func peerReadHandler(s string, raw []byte) int {
	msg := &proto_models.RawMessage{}
	err := proto.Unmarshal(raw, msg)
	if err != nil {
		verbose("parse failed: ", err)
		return 0
	}
	printMsg(msg)

	return len(raw)
}

func addPeerActions(conn *p2p.Conn, c *Client) {
	// note AddPeer blocks
	peer := conn.AddPeer(&c.Client)
	if peer == nil {
		log.Println("Client not acked: ", c)
		return
	}
	peer.SetReader(peerReadHandler)

	err := peer.WriteSerialized(NewMessage("Joined Conversation\n"))
	if err != nil {
		verbose("Error writing peer ack ", err)
	}
}

func main() {
	parseArgs()

	if h {
		flag.Usage()
		return
	}

	if v {
		verbose = log.Println
		if logf := openLogFile(); logf != nil {
			defer logf.Close()
		}
	}

	conn := p2p.NewConn()
	host, err := conn.ConnectHost(hostip)
	if err != nil {
		verbose("Error dial conn ", err)
		return
	}
	defer conn.Close()

	conn.AddSourcedHandler(
		hostip,
		func(raw []byte) (bRead int) {
			// verbose("raw: ", string(raw))
			c := &Client{}

			err := proto.Unmarshal(raw, c)
			if err != nil {
				verbose("not accespt: reason ", err)
				return
			}

			go addPeerActions(conn, c)

			// log.Println("adding ", c)

			bRead = len(raw)
			return
		},
	)

	// send a registeration information to host
	err = conn.Register()
	if err != nil {
		verbose("Register error ", err)
		return
	}
	defer signalExit(host)

	fmt.Println("* Local IP: ", conn.LocalIP)

	chatShellThread(conn)
}
