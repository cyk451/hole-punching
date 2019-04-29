OUT := ./bin
SRC := ./src
BINARIES := $(addprefix $(OUT)/, client server)

PROTO_SRC := client message
PROTO_SRC_LOC := ./proto
PROTO_TARGET_LOC := ./src/hpclient
PROTOS := $(addsuffix .proto, $(addprefix $(PROTO_SRC_LOC)/, $(PROTO_SRC)))
PROTO_TARGETS := $(addsuffix .pb.go, $(addprefix $(PROTO_TARGET_LOC)/, $(PROTO_SRC)))
# $(info PROTOS_TARGETS $(PROTO_TARGETS))

all: $(BINARIES)

$(BINARIES): | $(OUT)

$(OUT):
	-mkdir $(OUT)

$(OUT)/%: $(SRC)/%/*.go
	CGO_ENABLED=0 go build -o $@ $(SRC)/$(notdir $@)


# rebuild protos require installation of gogo-protobuf
protos: $(PROTO_TARGETS)

$(PROTO_TARGET_LOC)/%.pb.go: $(PROTO_SRC_LOC)/%.proto
	protoc -I=./$(PROTO_SRC_LOC) -I=$(GOPATH)/src/github.com/gogo/protobuf/protobuf --gogofaster_out=\
Mgoogle/protobuf/any.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/duration.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/struct.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/wrappers.proto=github.com/gogo/protobuf/types:$(PROTO_TARGET_LOC) \
		$<

clean:
	-rm $(BINARIES)
