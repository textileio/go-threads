module github.com/textileio/go-textile-thread

go 1.12

require (
	github.com/gogo/protobuf v1.2.1
	github.com/ipfs/go-ipld-cbor v0.0.3 // indirect
	github.com/multiformats/go-multihash v0.0.6 // indirect
	github.com/textileio/go-textile v0.6.3
	github.com/textileio/go-textile-core v0.0.1
)

replace github.com/textileio/go-textile-core v0.0.1 => ../go-textile-core/
