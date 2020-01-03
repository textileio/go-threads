PB = $(wildcard *.proto)
GO = $(PB:.proto=.pb.go)
PROTOC_GEN_TS_PATH = "./javascript/node_modules/.bin/protoc-gen-ts"
PROTOC_GEN_DART_PATH = "${HOME}/.pub-cache/bin/protoc-gen-dart"

all: $(GO)

%.pb.go: %.proto
	mkdir -p ./dart/lib && \
	protoc -I=. \
	--plugin="protoc-gen-ts=${PROTOC_GEN_TS_PATH}" --js_out="import_style=commonjs,binary:javascript/." --ts_out="service=grpc-web:javascript/." \
	--plugin="protoc-gen-dart=${PROTOC_GEN_DART_PATH}" --dart_out="grpc:dart/lib/." \
	--go_out=\
	plugins=grpc:\
	. $<

clean:
	rm -f *.pb.go
	rm -f *pb_test.go
	rm -f ./javascript/*.js
	rm -f ./javascript/*.d.ts
	rm -rf ./dart/lib

.PHONY: clean