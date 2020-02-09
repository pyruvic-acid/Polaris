GO := go install
GOTAG :=
GOARG = -tags '$(GOTAG)'
BIN := ${GOPATH}/bin
PACKAGE := Polaris

PROTODIR := src/rpc
PROTOSOURCE := $(wildcard $(PROTODIR)/*.proto)
PROTOBUF := $(patsubst $(PROTODIR)/%.proto, $(PROTODIR)/%.pb.go, $(PROTOSOURCE))

PROTOC := protoc
PROTOCFLAGS := -I $(PROTODIR) --go_out=plugins=grpc:$(PROTODIR)

OBJECTS := protobuf server client

all: $(OBJECTS)

nosign: GOTAG += nosign
nosign: all

full: clean $(OBJECTS)

clean:
	rm -f $(BIN)/polaris-server
	rm -f $(BIN)/polaris-client
	rm -f $(PROTOBUF)

protobuf: $(PROTOBUF)

$(PROTODIR)/%.pb.go: $(PROTODIR)/%.proto
	$(PROTOC) $(PROTOCFLAGS) $<

server: $(BIN)/polaris-server

$(BIN)/polaris-server: src/polaris-server/server.go
	$(GO) $(GOARG) $(PACKAGE)/src/polaris-server

client: polaris-client

polaris-client: $(BIN)/polaris-client

$(BIN)/polaris-client: src/polaris-client/client.go
	$(GO) $(GOARG) $(PACKAGE)/src/polaris-client

cleartmp:
	rm -rf raft-*
