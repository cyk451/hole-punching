OUT := ./bin
SRC := ./src
BINARIES := $(addprefix $(OUT)/, client server)

all: $(BINARIES)

$(BINARIES): | $(OUT)

$(OUT):
	-mkdir $(OUT)

$(OUT)/%: $(SRC)/%/*.go
	CGO_ENABLED=0 go build -o $@ $(SRC)/$(notdir $@)
