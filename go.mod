module github.com/textileio/go-textile-threads

go 1.12

require (
	github.com/ipfs/go-cid v0.0.2
	github.com/ipfs/go-ipld-cbor v0.0.3
	github.com/ipfs/go-ipld-format v0.0.2
	github.com/ipfs/go-log v0.0.1
	github.com/ipfs/go-merkledag v0.2.0
	github.com/libp2p/go-libp2p v0.2.0
	github.com/libp2p/go-libp2p-core v0.0.9
	github.com/libp2p/go-libp2p-peerstore v0.1.3
	github.com/multiformats/go-multiaddr v0.0.4
	github.com/multiformats/go-multihash v0.0.6
	github.com/polydawn/refmt v0.0.0-20190408063855-01bf1e26dd14
	github.com/textileio/go-textile-core v0.0.1
	github.com/textileio/go-textile-wallet v0.0.1
)

replace github.com/textileio/go-textile-core v0.0.1 => ../go-textile-core/
