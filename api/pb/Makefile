PB = $(wildcard *.proto)
GO = $(PB:.proto=.pb.go)
PROTOC_GEN_TS_PATH = "./node_modules/.bin/protoc-gen-ts"

all: $(GO)

%.pb.go: %.proto
	protoc -I=. -I=$(GOPATH)/src \
	--plugin="protoc-gen-ts=${PROTOC_GEN_TS_PATH}" --js_out="import_style=commonjs,binary:." --ts_out="service=grpc-web:." \
	--go_out=\
	plugins=grpc:\
	. $<

clean:
	rm -f *.pb.go
	rm -f *pb_test.go
	rm -f *.js
	rm -f *.d.ts

.PHONY: clean