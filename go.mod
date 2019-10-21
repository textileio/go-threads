module github.com/textileio/go-textile-threads

go 1.13

require (
	github.com/chzyer/logex v1.1.10 // indirect
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e
	github.com/chzyer/test v0.0.0-20180213035817-a1ea475d72b1 // indirect
	github.com/fatih/color v1.7.0
	github.com/gogo/protobuf v1.3.0
	github.com/hashicorp/golang-lru v0.5.3
	github.com/hsanjuan/ipfs-lite v0.1.6
	github.com/ipfs/go-block-format v0.0.2
	github.com/ipfs/go-blockservice v0.1.2
	github.com/ipfs/go-cid v0.0.3
	github.com/ipfs/go-datastore v0.1.1
	github.com/ipfs/go-ds-badger v0.0.5
	github.com/ipfs/go-ds-leveldb v0.1.0
	github.com/ipfs/go-ipfs-blockstore v0.1.0
	github.com/ipfs/go-ipfs-exchange-offline v0.0.1
	github.com/ipfs/go-ipld-cbor v0.0.3
	github.com/ipfs/go-ipld-format v0.0.2
	github.com/ipfs/go-log v0.0.1
	github.com/ipfs/go-merkledag v0.2.3
	github.com/libp2p/go-libp2p v0.4.0
	github.com/libp2p/go-libp2p-connmgr v0.1.1
	github.com/libp2p/go-libp2p-core v0.2.3
	github.com/libp2p/go-libp2p-gostream v0.2.0
	github.com/libp2p/go-libp2p-kad-dht v0.2.1
	github.com/libp2p/go-libp2p-peerstore v0.1.3
	github.com/libp2p/go-libp2p-pubsub v0.1.1
	github.com/mitchellh/go-homedir v1.1.0
	github.com/multiformats/go-multiaddr v0.1.1
	github.com/multiformats/go-multibase v0.0.1
	github.com/multiformats/go-multihash v0.0.8
	github.com/textileio/go-textile-core v0.0.0-20191016171609-e984eef83a4c
	github.com/whyrusleeping/base32 v0.0.0-20170828182744-c30ac30633cc
	github.com/whyrusleeping/go-logging v0.0.0-20170515211332-0457bb6b88fc
	google.golang.org/grpc v1.21.1
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
)

replace github.com/textileio/go-textile-core => ../go-textile-core/
