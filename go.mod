module github.com/textileio/go-textile-threads

go 1.13

require (
	github.com/gin-contrib/location v0.0.0-20190301062650-0462caccbb9c
	github.com/gin-gonic/gin v1.3.0
	github.com/gogo/protobuf v1.3.0
	github.com/hashicorp/golang-lru v0.5.3
	github.com/ipfs/go-block-format v0.0.2
	github.com/ipfs/go-cid v0.0.3
	github.com/ipfs/go-datastore v0.1.0
	github.com/ipfs/go-ds-badger v0.0.5
	github.com/ipfs/go-ds-leveldb v0.1.0
	github.com/ipfs/go-ipfs v0.4.22-0.20191002225611-b15edf287df6
	github.com/ipfs/go-ipld-cbor v0.0.3
	github.com/ipfs/go-ipld-format v0.0.2
	github.com/ipfs/go-log v0.0.1
	github.com/libp2p/go-libp2p v0.4.0
	github.com/libp2p/go-libp2p-core v0.2.3
	github.com/libp2p/go-libp2p-gostream v0.2.0
	github.com/libp2p/go-libp2p-peer v0.2.0
	github.com/libp2p/go-libp2p-peerstore v0.1.3
	github.com/libp2p/go-libp2p-pubsub v0.1.1
	github.com/multiformats/go-multiaddr v0.1.1
	github.com/multiformats/go-multihash v0.0.8
	github.com/pkg/errors v0.8.1
	github.com/rs/cors v1.7.0
	github.com/textileio/go-textile-core v0.0.1
	github.com/whyrusleeping/base32 v0.0.0-20170828182744-c30ac30633cc
	google.golang.org/grpc v1.20.1
)

replace github.com/textileio/go-textile-core v0.0.1 => ../go-textile-core/
