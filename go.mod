module github.com/textileio/go-threads

go 1.15

require (
	// agl/ed25519 only used in tests for backward compatibility, *do not* use in production code.
	github.com/agl/ed25519 v0.0.0-20170116200512-5312a6153412
	github.com/alecthomas/jsonschema v0.0.0-20191017121752-4bb6e3fae4f2
	github.com/desertbit/timer v0.0.0-20180107155436-c41aec40b27f // indirect
	github.com/dgraph-io/badger v1.6.2
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/dgtony/collections v0.1.6
	github.com/dlclark/regexp2 v1.2.0 // indirect
	github.com/dop251/goja v0.0.0-20200721192441-a695b0cdd498
	github.com/evanphx/json-patch v4.5.0+incompatible
	github.com/fsnotify/fsnotify v1.4.7
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	github.com/gogo/googleapis v1.3.1 // indirect
	github.com/gogo/protobuf v1.3.1
	github.com/gogo/status v1.1.0
	github.com/golang/protobuf v1.4.3
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.1
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/hsanjuan/ipfs-lite v1.1.17
	github.com/improbable-eng/grpc-web v0.13.0
	github.com/ipfs/go-bitswap v0.3.3 // indirect
	github.com/ipfs/go-block-format v0.0.2
	github.com/ipfs/go-blockservice v0.1.4
	github.com/ipfs/go-cid v0.0.7
	github.com/ipfs/go-datastore v0.4.5
	github.com/ipfs/go-ipfs-blockstore v1.0.3
	github.com/ipfs/go-ipfs-exchange-offline v0.0.1
	github.com/ipfs/go-ipfs-util v0.0.2
	github.com/ipfs/go-ipld-cbor v0.0.5
	github.com/ipfs/go-ipld-format v0.2.0
	github.com/ipfs/go-log v1.0.4
	github.com/ipfs/go-log/v2 v2.1.1
	github.com/ipfs/go-merkledag v0.3.2
	github.com/libp2p/go-libp2p v0.12.0
	github.com/libp2p/go-libp2p-connmgr v0.2.4
	github.com/libp2p/go-libp2p-core v0.7.0
	github.com/libp2p/go-libp2p-gostream v0.3.0
	github.com/libp2p/go-libp2p-kad-dht v0.11.0 // indirect
	github.com/libp2p/go-libp2p-noise v0.1.2 // indirect
	github.com/libp2p/go-libp2p-peerstore v0.2.6
	github.com/libp2p/go-libp2p-pubsub v0.4.0
	github.com/libp2p/go-libp2p-yamux v0.4.1 // indirect
	github.com/libp2p/go-netroute v0.1.4 // indirect
	github.com/libp2p/go-sockaddr v0.1.0 // indirect
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/multiformats/go-multibase v0.0.3
	github.com/multiformats/go-multihash v0.0.14
	github.com/multiformats/go-varint v0.0.6
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/namsral/flag v1.7.4-pre
	github.com/oklog/ulid/v2 v2.0.2
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/polydawn/refmt v0.0.0-20190807091052-3d65705ee9f1 // indirect
	github.com/rs/cors v1.7.0 // indirect
	github.com/smartystreets/goconvey v0.0.0-20190710185942-9d28bd7c0945 // indirect
	github.com/textileio/go-datastore-extensions v1.0.1
	github.com/textileio/go-ds-badger v0.2.7-0.20201204225019-4ee78c4a40e2
	github.com/textileio/go-ds-mongo v0.1.4
	github.com/tidwall/gjson v1.3.5
	github.com/tidwall/sjson v1.0.4
	github.com/whyrusleeping/base32 v0.0.0-20170828182744-c30ac30633cc
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonschema v1.2.0
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20201203163018-be400aefbc4c
	golang.org/x/exp v0.0.0-20200331195152-e8c3332aa8e5
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
	google.golang.org/grpc v1.31.1
	google.golang.org/protobuf v1.25.0 // indirect
)
