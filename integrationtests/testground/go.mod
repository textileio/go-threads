module github.com/textileio/go-threads/integrationtests/testground

go 1.15

require (
	github.com/go-kit/kit v0.10.0
	github.com/google/gofuzz v1.0.0
	github.com/guptarohit/asciigraph v0.5.2
	github.com/ipfs/go-cid v0.0.7
	github.com/ipfs/go-ipld-cbor v0.0.5
	github.com/ipfs/go-log/v2 v2.1.3
	github.com/libp2p/go-libp2p-core v0.8.5
	github.com/multiformats/go-multiaddr v0.3.3
	github.com/multiformats/go-multihash v0.0.15
	github.com/prometheus/client_golang v1.10.0
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/node_exporter v1.1.2
	github.com/testground/sdk-go v0.2.7
	github.com/textileio/go-threads v1.1.0-rc1.0.20210609142634-18b4ea73088f
	google.golang.org/grpc v1.37.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
)

replace github.com/textileio/go-threads v1.1.0-rc1.0.20210609142634-18b4ea73088f => github.com/mcrakhman/go-threads v1.1.0-rc1.0.20210626214637-1a88e4ef1d3b
