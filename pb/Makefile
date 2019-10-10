PB = $(wildcard *.proto)
GO = $(PB:.proto=.pb.go)

all: $(GO)

%.pb.go: %.proto
	protoc -I=. -I=$(GOPATH)/src --gogofaster_out=\
	plugins=grpc:\
	. $<

clean:
	rm -f *.pb.go

.PHONY: clean