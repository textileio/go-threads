# Datastore backed Threadstore

> Work pretty much inspired by `go-libp2p-peerstore` great implementation.

This is a `go-ipfs/go-datastore` backed implementation of [`go-textile-core/threadstore`](https://github.com/textileio/go-textile-core/blob/master/threadstore/threadstore.go):
* AddrBook (In progress)
* HeadBook (Pending)
* KeyBook (Pending)

For testing, two `go-ipfs/go-datastore` implementation are table-tested:
* [badger](github.com/ipfs/go-ds-badger)
* [leveldb](github.com/ipfs/go-ds-leveldb)

Tests leverage the `/test` suites used also for in-memory implementation. 