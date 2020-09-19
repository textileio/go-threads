# ThreadsDB

[![Made by Textile](https://img.shields.io/badge/made%20by-Textile-informational.svg?style=popout-square)](https://textile.io)
[![Chat on Slack](https://img.shields.io/badge/slack-slack.textile.io-informational.svg?style=popout-square)](https://slack.textile.io)
[![GitHub license](https://img.shields.io/github/license/textileio/go-threads.svg?style=popout-square)](./LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/textileio/go-threads?style=flat-square)](https://goreportcard.com/report/github.com/textileio/go-threads?style=flat-square)
[![GitHub action](https://github.com/textileio/go-threads/workflows/Tests/badge.svg?style=popout-square)](https://github.com/textileio/go-threads/actions)
[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg?style=popout-square)](https://github.com/RichardLitt/standard-readme)

> Server-less p2p database built on libp2p

Join us on our [public Slack channel](https://slack.textile.io/) for news, discussions, and status updates. [Check out our blog](https://medium.com/textileio) for the latest posts and announcements.

## Security

ThreadsDB is still under heavy development and no part of it should be used before a thorough review of the underlying code and an understanding APIs and protocols may change rapidly. There may be coding mistakes, and the underlying protocols may contain design flaws. Please [let us know](mailto:contact@textile.io) immediately if you have discovered a security vulnerability.

Please also read the [security note](https://github.com/ipfs/go-ipfs#security-issues) for [go-ipfs](https://github.com/ipfs/go-ipfs).

## Background

ThreadsDB is an implementation of the database described in the paper entitled [_A protocol & event-sourced database for decentralized user-siloed data_](https://docsend.com/view/gu3ywqi). 

Go to [the docs](https://docs.textile.io/) for more about the motivations behind ThreadsDB and Textile.

## Install

ThreadsDB has two distict layers:
-   ***`db`***:  The database layer is a document store, which internally leverages the `net` API. Most applications will only interface with this layer.
-   ***`net`***: The network layer maintains and orchestrates event logs between the network participants. Some applications, like event logging, may choose to rely on this layer directly.

This repo contains a daemon and client for interacting with these layers as a _remote service_. Depending on the application, Golang projects may choose to import the internal `db` and `net` packages directly.

### Daemon

-   **Prebuilt package**: See [release assets](https://github.com/textileio/go-threads/releases/latest)
-   **Docker image**: See the `latest` tag on [Docker Hub](https://hub.docker.com/r/textile/go-threads/tags)
-   **Build from the source**:

```
git clone https://github.com/textileio/go-threads
cd go-threads
go get ./threadsd
```

#### Client

```
import "github.com/textileio/go-threads/api/client"
```

## Getting Started 

### Running ThreadsDB

### The DB API

The full API spec is available [here](https://github.com/textileio/go-threads/blob/master/api/pb/threads.proto).

As described in the [paper](https://docsend.com/view/gu3ywqi), ThreadDB's network layer orchestrates groups of event logs, or _threads_. In the current implementation, a single database leverage a single network-layer thread for state orchestration.

#### Getting a thread token

#### Creating a new DB

#### Creating a new DB from an existing address

#### Creating a Collection

#### Updating a Collection

#### Creating a Collection instance

#### Saving a Collection instance

#### Collection queries

#### Transactions

#### Listening for collection changes

### The Network API

The full API spec is available [here](https://github.com/textileio/go-threads/blob/master/net/api/pb/threadsnet.proto).

#### Getting a thread token

#### Creating a thread

#### Adding an existing thread

#### Adding a thread replicator

#### Creating a thread record

#### Adding an existing record

#### Pulling a thread for updates

#### Listening for thread updates

## Contributing

Pull requests and bug reports are very welcome ❤️

This repository falls under the Textile [Code of Conduct](./CODE_OF_CONDUCT.md).

Feel free to get in touch by:
-   [Opening an issue](https://github.com/textileio/go-threads/issues/new)
-   Joining the [public Slack channel](https://slack.textile.io/)
-   Sending an email to contact@textile.io

## Changelog

A changelog is published along with each [release](https://github.com/textileio/go-threads/releases).

## License

[MIT](LICENSE)
