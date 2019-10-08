PB = $(wildcard *.proto)
GO = $(PB:.proto=.pb.go)

all: $(GO)

%.pb.go: %.proto
	protoc -I=. -I=$(GOPATH)/src --gogofaster_out=\
	Mgoogle/protobuf/struct.proto=github.com/gogo/protobuf/types,\
	Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,\
	plugins=grpc:\
	. $<

clean:
	rm -f *.pb.go

.PHONY: clean