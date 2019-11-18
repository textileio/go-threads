module github.com/textileio/go-textile-threads

go 1.13

require (
	github.com/AndreasBriese/bbloom v0.0.0-20190823232136-616930265c33 // indirect
	github.com/alecthomas/jsonschema v0.0.0-20191017121752-4bb6e3fae4f2
	github.com/evanphx/json-patch v4.5.0+incompatible
	github.com/fatih/color v1.7.0
	github.com/gogo/protobuf v1.3.0
	github.com/gogo/status v1.1.0
	github.com/gorilla/websocket v1.4.1
	github.com/hashicorp/golang-lru v0.5.3
	github.com/hsanjuan/ipfs-lite v0.1.6
	github.com/improbable-eng/grpc-web v0.11.0
	github.com/ipfs/go-block-format v0.0.2
	github.com/ipfs/go-blockservice v0.1.2
	github.com/ipfs/go-cid v0.0.3
	github.com/ipfs/go-datastore v0.1.1
	github.com/ipfs/go-ds-badger v0.0.5
	github.com/ipfs/go-ds-leveldb v0.1.0 // indirect
	github.com/ipfs/go-ipfs-blockstore v0.1.0
	github.com/ipfs/go-ipfs-exchange-offline v0.0.1
	github.com/ipfs/go-ipfs-files v0.0.4 // indirect
	github.com/ipfs/go-ipld-cbor v0.0.3
	github.com/ipfs/go-ipld-format v0.0.2
	github.com/ipfs/go-log v0.0.1
	github.com/ipfs/go-merkledag v0.2.3
	github.com/libp2p/go-libp2p v0.4.0
	github.com/libp2p/go-libp2p-connmgr v0.1.1
	github.com/libp2p/go-libp2p-core v0.2.3
	github.com/libp2p/go-libp2p-gostream v0.2.0
	github.com/libp2p/go-libp2p-kad-dht v0.3.0
	github.com/libp2p/go-libp2p-peerstore v0.1.3
	github.com/libp2p/go-libp2p-pubsub v0.1.1
	github.com/libp2p/go-libp2p-swarm v0.2.2
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mr-tron/base58 v1.1.2
	github.com/multiformats/go-multiaddr v0.1.1
	github.com/multiformats/go-multibase v0.0.1
	github.com/multiformats/go-multihash v0.0.8
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/rs/cors v1.7.0 // indirect
	github.com/textileio/go-textile-core v0.0.0-20191112000026-958d7d17affc
	github.com/whyrusleeping/base32 v0.0.0-20170828182744-c30ac30633cc
	github.com/whyrusleeping/go-logging v0.0.0-20170515211332-0457bb6b88fc
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonschema v1.2.0
	golang.org/x/crypto v0.0.0-20190926180335-cea2066c6411 // indirect
	golang.org/x/net v0.0.0-20191014212845-da9a3fd4c582 // indirect
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20190926180325-855e68c8590b // indirect
	google.golang.org/genproto v0.0.0-20190502173448-54afdca5d873 // indirect
	google.golang.org/grpc v1.24.0
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
)

replace github.com/textileio/go-textile-core => ../go-textile-core/
