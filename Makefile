OUT_DIR := ./bin
SRC_DIR := ./src
BINARIES := $(addprefix $(OUT_DIR)/, client server)

# $(info PROTOS_TARGETS $(PROTO_TARGETS))

all: $(BINARIES)

$(BINARIES): | $(OUT_DIR)

$(OUT_DIR):
	-mkdir $(OUT_DIR)

$(OUT_DIR)/%: $(SRC_DIR)/%/*.go
	CGO_ENABLED=0 go build -o $@ $(SRC_DIR)/$(notdir $@)


PROTO_SRC := client message command
PROTO_SRC_LOC := ./proto
PROTO_TARGET_LOC := ./src/proto_models
PROTOS := $(addsuffix .proto, $(addprefix $(PROTO_SRC_LOC)/, $(PROTO_SRC)))
PROTO_TARGETS := $(addsuffix .pb.go, $(addprefix $(PROTO_TARGET_LOC)/, $(PROTO_SRC)))
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
