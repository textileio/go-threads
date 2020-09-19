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

ThreadsDB has two distinct layers:

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

You can think of the [DB client](github.com/textileio/go-threads/api/client) as a gRPC client wrapper around the internal `db` package API, and the [Network client](github.com/textileio/go-threads/net/api/client) as a gRPC client wrapper around the internal `net` package API. This section will only focus on getting started with the gRPC clients, but Golang apps may choose to interact directly with `db` and/or `net`.

### Running ThreadsDB

The `threadsd` daemon can be run as a server or alongside desktop apps or command-line tools. The easiest way to run `threadsd` is by using the provided Docker Compose files. If you're new to Docker and/or Docker Compose, get started [here](https://docs.docker.com/compose/gettingstarted/). Once you are setup, you should have `docker-compose` in your `PATH`.

Create an `.env` file and add the following values:  

```
REPO_PATH=~/myrepo
THRDS_DEBUG=true
```

Copy [this compose file](https://github.com/textileio/go-threads/blob/master/docker-compose.yml) and run it with the following command.

```
docker-compose -f docker-compose.yml up 
```

You should see some console output:

```
threads_1  | 2020-09-19T16:34:06.420Z	DEBUG	threadsd	repo: /data/threads
threads_1  | 2020-09-19T16:34:06.420Z	DEBUG	threadsd	hostAddr: /ip4/0.0.0.0/tcp/4006
threads_1  | 2020-09-19T16:34:06.421Z	DEBUG	threadsd	apiAddr: /ip4/0.0.0.0/tcp/6006
threads_1  | 2020-09-19T16:34:06.421Z	DEBUG	threadsd	apiProxyAddr: /ip4/0.0.0.0/tcp/6007
threads_1  | 2020-09-19T16:34:06.421Z	DEBUG	threadsd	connLowWater: 100
threads_1  | 2020-09-19T16:34:06.421Z	DEBUG	threadsd	connHighWater: 400
threads_1  | 2020-09-19T16:34:06.422Z	DEBUG	threadsd	connGracePeriod: 20s
threads_1  | 2020-09-19T16:34:06.423Z	DEBUG	threadsd	keepAliveInterval: 5s
threads_1  | 2020-09-19T16:34:06.423Z	DEBUG	threadsd	enableNetPubsub: false
threads_1  | 2020-09-19T16:34:06.424Z	DEBUG	threadsd	debug: true
threads_1  | Welcome to Threads!
threads_1  | Your peer ID is 12D3KooWFCXqmQTwvpfYFWK3DjXChEc4NoPt8pp5jjC8REZ3g6NZ
```

Congrats! Now you have ThreadsDB running locally.

#### Configuration values

Note the various configuration values shown in the output above. These can be modified with environment variables show below.

-   ***`THRDS_REPO`***: Repo location. `.threads` by default.
-   ***`THRDS_HOSTADDR`***: Libp2p host bind address. `/ip4/0.0.0.0/tcp/4006` by default.
-   ***`THRDS_APIADDR`***: gRPC API bind address. `/ip4/0.0.0.0/tcp/6006` by default.
-   ***`THRDS_APIPROXYADDR`***: gRPC API web proxy bind address. `/ip4/0.0.0.0/tcp/6007` by default.
-   ***`THRDS_CONNLOWWATER`***: Low watermark of libp2p connections that'll be maintained. `100` by default.
-   ***`THRDS_CONNHIGHWATER`***: High watermark of libp2p connections that'll be maintained. `400` by default.
-   ***`THRDS_CONNGRACEPERIOD`***: Duration a new opened connection is not subject to pruning. `20` seconds by default.
-   ***`THRDS_KEEPALIVEINTERVAL`***: Websocket keepalive interval (must be >= 1s). `5` seconds by default.
-   ***`THRDS_ENABLENETPUBSUB`***: Enables thread networking over libp2p pubsub. `false` by default.
-   ***`THRDS_DEBUG`***: Enables debug logging. `false` by default.

### The DB API

The full API spec is available [here](https://github.com/textileio/go-threads/blob/master/api/pb/threads.proto).

As described in the [paper](https://docsend.com/view/gu3ywqi), ThreadDB's network layer orchestrates groups of event logs, or _threads_. In the current implementation, a single database leverages a single network-layer thread for state orchestration.

#### Starting the client

```
import "github.com/textileio/go-threads/api/client"
...

db, err := client.NewClient("/ip4/127.0.0.1/tcp/6006", grpc.WithInsecure())

```

#### Getting a thread token

```
privateKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
identity := thread.NewLibp2pIdentity(privateKey)

myToken, err := db.GetToken(context.Background(), identity)
```

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
