# Datastore backed Logstore

> Work pretty much inspired by `go-libp2p-peerstore` great implementation.

This is a `go-datastore` backed implementation of [`go-threads/core/logstore`](https://github.com/textileio/go-threads/blob/master/core/logstore/logstore.go):
* AddrBook
* HeadBook
* KeyBook

For testing, two `go-datastore` implementation are table-tested:
* [badger](github.com/ipfs/go-ds-badger)
* [leveldb](github.com/ipfs/go-ds-leveldb)

Tests leverage the `/test` suites used also for in-memory implementation. 