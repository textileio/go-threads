PB = $(wildcard *.proto)
GO = $(PB:.proto=.pb.go)
PROTOC_GEN_TS_PATH = "./node_modules/.bin/protoc-gen-ts"

all: $(GO)

%.pb.go: %.proto
	protoc -I=. -I=$(GOPATH)/src \
	--plugin="protoc-gen-ts=${PROTOC_GEN_TS_PATH}" --js_out="import_style=commonjs,binary:." --ts_out="." \
	--gogofaster_out=\
	plugins=grpc:\
	. $<

clean:
	rm -f *.pb.go
	rm -f *.pb.js
	rm -f *.pb.d.ts

.PHONY: clean