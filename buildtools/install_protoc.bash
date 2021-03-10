#!/bin/bash
set -eo pipefail

if [ ! -d ./protoc ]; then
	OS=$(uname)
	if [ $OS = "Darwin" ]; then
		OS="osx"
	fi
	VERSION=3.13.0
	ZIPNAME=protoc-$VERSION-$OS-x86_64
	DOWNLOADLINK=https://github.com/protocolbuffers/protobuf/releases/download/v$VERSION/$ZIPNAME.zip
	curl -LO $DOWNLOADLINK
	unzip $ZIPNAME.zip -d protoc
	rm $ZIPNAME.zip
fi

if [ ! -d ./protoc-gen-go ]; then
	git clone --single-branch --depth 1 --branch "v1.4.3" https://github.com/golang/protobuf.git
	cd protobuf 
	go build -o ../protoc-gen-go/protoc-gen-go ./protoc-gen-go 
	cd ..
	rm -rf protobuf
fi
