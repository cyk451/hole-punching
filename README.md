### Hole Punching Chat Server

This is a simple udp server implements a common technique used to accomplish
P2P connections known as hole punching. This topic is elaborated here on the [Wiki](https://en.wikipedia.org/wiki/Hole_punching_(networking)).

Through the udp, this tiny console chatting application can connect its peers
behind NATs. But as described in the wiki, this approach relies on certain
behaviours from NATs, and do not applicable to some, so-called symmetric, NATs.
Anyway, this program is purely for fun, so the reliability won't be really
matter.

# File Hierachy

```
.
├── client
│   └── main.go
├── Dockerfile
├── go.mod
├── go.sum
├── Makefile
├── p2p
│   ├── conn.go
│   └── ios.go
├── proto
│   ├── client.proto
│   ├── command.proto
│   └── message.proto
├── proto_models
│   ├── client.pb.go
│   ├── command.pb.go
│   └── message.pb.go
├── README.md
└── server
    └── main.go
```

# Build and Deploy

Build machine:
```bash
make

docker build -t $HP_HOST_IP:5000/hp-server .

docker push $HP_HOST_IP:5000/hp-server

```

Deployment on server machine:
```bash
docker pull $HP_HOST_IP:5000/hp-server

docker run -d -p 11711:11711udp -n hp-server $HP_HOST_IP:5000/hp-server

```

Client side connection:
```bash
./bin/client -H $HP_HOST_IP -l client.log
```

