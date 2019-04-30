package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/cyk451/hole-punching/src/p2p"
	"github.com/cyk451/hole-punching/src/proto_models"
)

type Client = proto_models.Client
type IdGenerator = func() uint32
type Peer = p2p.PeerIO

type P2PServer struct {
	*p2p.Conn
	/* keys are public ips, which should be unique */
	peerTable map[string]*Peer
	idGen     IdGenerator
}

const Version string = "0.1"

/* arguments */
var (
	h      bool
	v      bool
	logfn  string
	hostip string
)

// dummy verbose function
var verbose func(...interface{}) = func(...interface{}) {}

func parseArgs() {
	flag.BoolVar(&h, "h", false,
		"print this help and quit")
	flag.BoolVar(&v, "v", false,
		"be verbose")
	flag.BoolVar(&v, "V", false,
		"print software version and quit")
	flag.StringVar(&hostip, "H", "0.0.0.0:11711",
		"ip listening for up coming requests")
	flag.StringVar(&logfn, "l", "STDERR",
		"a filename to which logs are printed. used only if verbose enabled")

	flag.Parse()
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

/* use store interfaces */

func NewP2PServer() *P2PServer {
	return &P2PServer{
		Conn:      p2p.NewConn(),
		peerTable: make(map[string]*Peer),
		// Note it count from 1
		idGen: func() IdGenerator {
			i := uint32(0)
			return func() uint32 {
				i += 1
				return i
			}
		}(),
	}
}

func (p2p *P2PServer) hasPeer(public string) bool {
	_, ok := p2p.peerTable[public]
	return ok
}

func (p2p *P2PServer) respondList(np *Peer) error {
	for i, p := range p2p.peerTable {
		if i == np.IP() {
			continue
		}

		err := np.WriteSerialized(p.Client)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p2p *P2PServer) notifyNewPeer(np *Peer) error {
	for i, p := range p2p.peerTable {
		if i == np.IP() {
			continue
		}

		err := p.WriteSerialized(np.Client)
		if err != nil {
			return err
		}
	}
	return nil
}

func addPeerActions(p2p *P2PServer, c *Client) {
	peer := p2p.AddPeer(c)

	if peer == nil {
		log.Println("Client ", c.Public, " adding failed, no responding")
		return
	}
	// TODO adding semephore
	p2p.respondList(peer)

	p2p.notifyNewPeer(peer)

	p2p.peerTable[c.Public] = peer

	log.Println("We now have ", len(p2p.peerTable), " clients")
}

func (p2p *P2PServer) handleRegisteration(public string, private string) {

	if p2p.hasPeer(public) {
		log.Println("This public ip is already registered. Do nothing.")
		return
	}

	temp := strings.Split(public, ":")
	if len(temp) != 2 {
		log.Println("public ip should have port")
		return
	}

	log.Println("Adding ", public, " to peer list")

	if private == public {
		private = "" // i think we can save a duplication here.
	}

	go addPeerActions(p2p, &Client{
		Public:  public,
		Private: private,
		Id:      p2p.idGen(),
	})
}

func (p2p *P2PServer) handleClose(public string) {
	log.Println("Removing peer ", public)
	delete(p2p.peerTable, public)
	// TODO implement client interface to remove peer
}

func handlerBindP2P(p2p *P2PServer) p2p.Handler {
	return func(source string, what []byte) (bRead int) {
		toks := strings.Split(string(what), " ")

		cmd := toks[0]
		switch cmd {
		case "register":

			// register <private ip address>
			// register itself to be a p2p client, respond a client struct
			// which is registered client
			if len(toks) < 2 {
				log.Println("command register require peer private ip address...")
				return 0
			}
			p2p.handleRegisteration(source, toks[1])
		case "close":
			p2p.handleClose(source)
		default:
			log.Println("Unknown command from ", source, ": ", cmd)
		}
		return len(what)
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
		logf := openLogFile()
		if logf != nil {
			defer logf.Close()
		}
	}

	p2p := NewP2PServer()

	p2p.AddHandler(handlerBindP2P(p2p))

	log.Println("Listening ", hostip)
	err := p2p.Listen(hostip)
	if err != nil {
		log.Println("listen: ", err)
	}
}
