PB = $(wildcard *.proto)
GO = $(PB:.proto=.pb.go)
PROTOC_GEN_TS_PATH = "./javascript/node_modules/.bin/protoc-gen-ts"

all: $(GO)

%.pb.go: %.proto
	protoc -I=. -I=$(GOPATH)/src/github.com/gogo/protobuf/protobuf -I=$(GOPATH)/src \
	--plugin="protoc-gen-ts=${PROTOC_GEN_TS_PATH}" --js_out="import_style=commonjs,binary:javascript/." --ts_out="service=grpc-web:javascript/." \
	--gogofaster_out=\
	plugins=grpc:\
	. $<

clean:
	rm -f *.pb.go
	rm -f *pb_test.go
	rm -f ./javascript/*.js
	rm -f ./javascript/*.d.ts
	rm -rf ./javascript/github.com

.PHONY: clean

gogo:
	protoc -I=. -I=$(GOPATH)/src/github.com/gogo/protobuf/protobuf -I=$(GOPATH)/src \
	--plugin="protoc-gen-ts=${PROTOC_GEN_TS_PATH}" --js_out="import_style=commonjs,binary:javascript/." --ts_out="javascript/." \
	--gogofaster_out=\
	. $(GOPATH)/src/github.com/gogo/protobuf/gogoproto/gogo.proto
	rm -rf github.com

.PHONY: gogo
