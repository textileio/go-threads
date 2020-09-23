# ThreadDB

[![Made by Textile](https://img.shields.io/badge/made%20by-Textile-informational.svg?style=popout-square)](https://textile.io)
[![Chat on Slack](https://img.shields.io/badge/slack-slack.textile.io-informational.svg?style=popout-square)](https://slack.textile.io)
[![GitHub license](https://img.shields.io/github/license/textileio/go-threads.svg?style=popout-square)](./LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/textileio/go-threads?style=flat-square)](https://goreportcard.com/report/github.com/textileio/go-threads?style=flat-square)
[![GitHub action](https://github.com/textileio/go-threads/workflows/Tests/badge.svg?style=popout-square)](https://github.com/textileio/go-threads/actions)
[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg?style=popout-square)](https://github.com/RichardLitt/standard-readme)

> Server-less p2p database built on libp2p

Join us on our [public Slack channel](https://slack.textile.io/) for news, discussions, and status updates. [Check out our blog](https://medium.com/textileio) for the latest posts and announcements.

## Table of Contents

* [Table of Contents](#table-of-contents)
* [Security](#security)
* [Background](#background)
* [Install](#install)
  * [Daemon](#daemon)
  * [Client](#client)
* [Getting Started](#getting-started)
  * [Running ThreadDB](#running-threaddb)
    * [Configuration values](#configuration-values)
  * [The DB API](#the-db-api)
    * [Starting the client](#starting-the-client)
    * [Getting a thread token](#getting-a-thread-token)
    * [Creating a new DB](#creating-a-new-db)
    * [Creating a new DB from an existing address](#creating-a-new-db-from-an-existing-address)
    * [Creating a collection](#creating-a-collection)
      * [Write Validation](#write-validation)
      * [Read Filtering](#read-filtering)
    * [Updating a collection](#updating-a-collection)
    * [Creating an instance](#creating-an-instance)
    * [Saving an instance](#saving-an-instance)
    * [Collection queries](#collection-queries)
    * [Transactions](#transactions)
      * [Write transactions](#write-transactions)
      * [Read transactions](#read-transactions)
    * [Listening for collection changes](#listening-for-collection-changes)
  * [The Network API](#the-network-api)
    * [Starting the client](#starting-the-client-1)
    * [Getting a thread token](#getting-a-thread-token-1)
    * [Creating a thread](#creating-a-thread)
    * [Adding an existing thread](#adding-an-existing-thread)
    * [Adding a thread replicator](#adding-a-thread-replicator)
    * [Creating a thread record](#creating-a-thread-record)
    * [Adding a thread record](#adding-a-thread-record)
    * [Pulling a thread for new records](#pulling-a-thread-for-new-records)
    * [Listening for new records](#listening-for-new-records)
* [Developing](#developing)
* [Contributing](#contributing)
* [Changelog](#changelog)
* [License](#license)

## Security

ThreadDB is still under heavy development and no part of it should be used before a thorough review of the underlying code and an understanding APIs and protocols may change rapidly. There may be coding mistakes, and the underlying protocols may contain design flaws. Please [let us know](mailto:contact@textile.io) immediately if you have discovered a security vulnerability.

Please also read the [security note](https://github.com/ipfs/go-ipfs#security-issues) for [go-ipfs](https://github.com/ipfs/go-ipfs).

## Background

ThreadDB is an implementation of the database described in the paper entitled [_A protocol & event-sourced database for decentralized user-siloed data_](https://docsend.com/view/gu3ywqi). 

Go to [the docs](https://docs.textile.io/) for more about the motivations behind ThreadDB and Textile.

## Install

ThreadDB has two distinct layers:

-   ***`db`***:  The database layer is a document store, which internally leverages the `net` API. Most applications will only interface with this layer.
-   ***`net`***: The network layer maintains and orchestrates append-only event logs between network participants. Some applications, like event logging, may choose to rely on this layer directly.

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

### Running ThreadDB

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

Congrats! Now you have ThreadDB running locally.

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

The database layer is a document store, which internally leverages the `net` API. Most applications will only interface with this layer.

The full API spec is available [here](https://pkg.go.dev/github.com/textileio/go-threads/api/client).

As described in the [paper](https://docsend.com/view/gu3ywqi), ThreadDB's network layer orchestrates groups of event logs, or _threads_. In the current implementation, a single database leverages a single network-layer thread for state orchestration.

#### Starting the client

```
import "github.com/textileio/go-threads/api/client"
...

db, err := client.NewClient("/ip4/127.0.0.1/tcp/6006", grpc.WithInsecure())
```

#### Getting a thread token

Thread _tokens_ ([JWTs](https://jwt.io/)) are used by the daemon to determine the _indentity_ of the caller. Most APIs take a thread token as an optional argument, since whether or not they are needed usually depends on how the target collection is configured (see [Write Validation](#write-validation) and [Read Filtering](#read-filtering)). These tokens are obtained by performing a signing challenge with the daemon using a libp2p private key.

```
privateKey, _, err := crypto.GenerateEd25519Key(rand.Reader) // Private key is kept locally
myIdentity := thread.NewLibp2pIdentity(privateKey)

threadToken, err := db.GetToken(context.Background(), myIdentity)
```

#### Creating a new DB

```
threadID := thread.NewIDV1(thread.Raw, 32)
err := db.NewDB(context.Background(), threadID)
```

#### Creating a new DB from an existing address

An existing DB can be added to a different daemon by providing a valid host address and thread key.

```
threadID := thread.NewIDV1(thread.Raw, 32)
err := db1.NewDB(context.Background(), threadID)
dbInfo, err := db1.GetDBInfo(context.Background(), threadID)
...

// db2 is a different client (this would normally be done on a different machine)
err := db2.NewDBFromAddr(context.Background(), dbInfo.Addrs[0], dbInfo.Key)
```

#### Creating a collection

Collections are groups of documents or _instances_ and are analogous to tables in relational databases. Creating a collection involves defining the following configuration parameters:

-   ***`Name`***: The name of the collection, e.g, "Animals" (must be unique per DB).
-   ***`Schema`***: A [JSON Schema](https://json-schema.org/)), which is used for instance validation.
-   ***`Indexes`***: An optional list of index configurations, which define how instances are indexed.
-   ***`WriteValidator`***: An optional JavaScript (ECMAScript 5.1) function that is used to validate instances on write.
-   ***`ReadFilter`***: An optional JavaScript (ECMAScript 5.1) function that is used to filter instances on read.

##### Write Validation

The `WriteValidator` function receives three arguments:

-   `writer`: The multibase-encoded public key identity of the writer.
-   `event`: An object describing the update event (see [`core.Event`](https://pkg.go.dev/github.com/textileio/go-threads/core/db#Event)).
-   `instance`: The current instance as a JavaScript object before the update event is applied.

A [falsy](https://developer.mozilla.org/en-US/docs/Glossary/Falsy) return value indicates a failed validation.

Having access to `writer`, `event`, and `instance` opens the door to a variety of app-specific logic. Textile Buckets file-level access roles are implemented in part with a write validator.

##### Read Filtering

The function receives three arguments:

-   `reader`: The multibase-encoded public key identity of the reader.
-   `instance`: The current instance as a JavaScript object.

The function must return a JavaScript object. Most implementation will modify and return the current instance.

Like write validation, read filtering opens the door to a variety of app-specific logic. Textile Buckets file-level access roles are implemented in part with a read filter.

```
import "github.com/alecthomas/jsonschema"
...

// We can use a struct to define a collection schema
type Person struct {
    ID        string `json:"_id"`
    Name      string `json:"name"`
    Age       int    `json:"age"`
    CreatedAt int    `json:"created_at"`
}

reflector := jsonschema.Reflector{}
mySchema = reflector.Reflect(&Person{}) // Generate a JSON Schema from a struct

err := db.NewCollection(context.Background(), myThreadID, db.CollectionConfig{
    Name:    "Persons",
    Schema:  mySchema,
    Indexes: []db.Index{{
        Path:   "name", // Value matches json tags
		Unique: true, // Create a unique index on "name"
    }},
})

...

// We can use the same schema to create more collections.
err := db.NewCollection(context.Background(), myThreadID, db.CollectionConfig{
    Name:    "Persons",
    Schema:  mySchema,
    Indexes: []db.Index{{
        Path:   "name",
		Unique: true,
    }},
    WriteValidator: `
        var type = event.patch.type
        var patch = event.patch.json_patch
        switch (type) {
          case "delete":
            if (writer != "the_boss") {
              return false // Not the boss? No deletes for you.
            }
          default:
            return true
        }
    `,
    ReadFilter: `
        if (instance.Age > 50) {
            delete instance.Age // Getting old, let's hide just _how_ old hehe
        }
        return instance
    `,
})
```

#### Updating a collection

Each of the collection configuration parameters above can be updated.

```
...

err := db.UpdateCollection(context.Background(), myThreadID, db.CollectionConfig{
    Name:    "Persons",
    Schema:  mySchema,
    Indexes: []db.Index{{
        Path:   "name",
        Unique: true,
    },
    {
        Path: "created_at", // Add an additional index on "created_at"
    }},
})
```

#### Creating an instance

Creating a collection instance is analogous to inserting a row in a relational database table.

```
...

// ID is autogenerated when omitted
alice := &Person{
    ID:        "",
    Name:      "Alice",
    Age:       30,
    CreatedAt: time.Now().UnixNano(),
}

ids, err := db.Create(context.Background(), threadID, "Persons", Instances{alice})

alice.ID = ids[0] // ids contains autogenerated instance identifiers

// We can also define a custom ID, it just has to be a collection-wide unique string
bob := &Person{
    ID:        "123",
    Name:      "Bob",
    Age:       30,
    CreatedAt: time.Now().UnixNano(),
}

ids, err := db.Create(context.Background(), threadID, "Persons", Instances{bob})
```

#### Saving an instance

Similarly, we can update an instance with new values.

```
...

alice.Age = 31
err = db.Save(context.Background(), threadID, "Persons", Instances{alice})
```

#### Collection queries

There are three methods to query for collection instances: ***`Find`***, ***`FindByID`***, and ***`Has`***. As usual, queries are enhanced by indexes.

Check out [`db.Query`](https://pkg.go.dev/github.com/textileio/go-threads/db#Query) and [`db.Criterion`](https://pkg.go.dev/github.com/textileio/go-threads/db#Criterion) for more about constructing queries and ordering results.

```
...

// Find instances with a query
query := db.Where("name").Eq("Alice")
results, err := db.Find(context.Background(), threadID, "Persons", query, &Person{})

alice := results[0].(*Person)

...

// Find an instance by ID
alice := &Person{}
err = db.FindByID(context.Background(), threadID, "Persons", aliceID, alice)

...

// Determine if an instance exists by ID
exists, err := db.Has(context.Background(), threadID, "Persons", []string{aliceID})
```

#### Transactions

ThreadDB transactions come in two flavors: `WriteTransaction` and `ReadTransaction`.

##### Write transactions

```
...

txn, err := db.WriteTransaction(context.Background(), threadID, "Persons")
end, err := txn.Start()
defer end()

alice.Age = 32
err = txn.Save(alice)

err = txn.Create(&Person{
    Name:      "Bob",
    Age:       30,
    CreatedAt: time.Now().UnixNano(),
})

end() // Done writing, commit transaction updates
```

##### Read transactions

```
...

txn, err := db.ReadTransaction(context.Background(), threadID, "Persons")
end, err := txn.Start()

hasAlice, err := txn.Has(alice.ID)

results, err := txn.Find(db.Where("name").Eq("Bob"), &Person{})

bob := results[0].(*Person)

end() // Done reading
```

#### Listening for collection changes

We can listen for DB changes on three levels: DB, collection, or instance.

Check out [ListenOption](https://pkg.go.dev/github.com/textileio/go-threads/api/client#ListenOption) for more.

```
...

ctx, cancel := context.WithCancel(context.Background())
defer cancel()
events, err := db.Listen(ctx, threadID, []db.ListenOption{{
    Type: client.ListenAll,
    Collection: "Persons",  // Omit to receive events from all collections
    InstanceID: bob.ID,     // Omit to receive events from all instances
}})

for event := range events {
    // Handle event
}
```

### The Network API

The network layer maintains and orchestrates append-only event logs between network participants and is used internally by the database layer. Some applications, like event logging, may choose to rely on this layer directly.

The full API spec is available [here](https://pkg.go.dev/github.com/textileio/go-threads/net/api/client).

#### Starting the client

```
import "github.com/textileio/go-threads/net/api/client"
...

net, err := client.NewClient("/ip4/127.0.0.1/tcp/6006", grpc.WithInsecure())
```

#### Getting a thread token

Thread _tokens_ ([JWTs](https://jwt.io/)) are used by the daemon to determine the _indentity_ of the caller. Most APIs take a thread token as an optional argument.

```
privateKey, _, err := crypto.GenerateEd25519Key(rand.Reader) // Private key is kept locally
myIdentity := thread.NewLibp2pIdentity(privateKey)

threadToken, err := net.GetToken(context.Background(), myIdentity)
```

#### Creating a thread

```
threadID := thread.NewIDV1(thread.Raw, 32)
threadInfo, err := net.CreateThread(context.Background(), threadID)
```

#### Adding an existing thread

An existing thread can be added to a different daemon by providing a valid host address and thread key.

```
threadID := thread.NewIDV1(thread.Raw, 32)
threadInfo1, err := net1.CreateThread(context.Background(), threadID)
...

// net2 is a different client (this would normally be done on a different machine)
threadInfo2, err := net2.AddThread(context.Background(), threadInfo.Addrs[0], core.WithThreadKey(threadInfo1.Key))
```

#### Adding a thread replicator

We can replicate a thread on a different host. All logs and records are pushed to the new host. However, it will not be able to read them since it won't receive _read_ portion of the thread key.

```
threadID := thread.NewIDV1(thread.Raw, 32)
threadInfo, err := net1.CreateThread(context.Background(), threadID)

replicatorAddr, err := multiaddr.NewMultiaddr("/ip4/<REPLICATOR_IP_ADDRESS>/tcp/4006/p2p/<REPLICATOR_PEER_ID>")
replicatorID, err := net.AddReplicator(context.Background(), threadID, replicatorAddr)
```

#### Creating a thread record

A thread record can have any body.

```
import ipldcbor "github.com/ipfs/go-ipld-cbor"
...

body, err := ipldcbor.WrapObject(map[string]interface{}{
    "foo": "bar",
    "baz": []byte("howdy"),
}, multihash.SHA2_256, -1)
	
record, err := net.CreateRecord(context.Background(), threadID, body)
```

#### Adding a thread record

We can add also retain control over the _read_ portion of the thread key and the log private key and create records _locally_.

```
import ipldcbor "github.com/ipfs/go-ipld-cbor"
...

privateKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
myIdentity := thread.NewLibp2pIdentity(privateKey)

threadToken, err := net.GetToken(context.Background(), myIdentity)

threadID := thread.NewIDV1(thread.Raw, 32)
threadKey := thread.NewRandomKey()
logPrivateKey, logPublicKey, err := crypto.GenerateEd25519Key(rand.Reader)
logID, err := peer.IDFromPublicKey(logPublicKey)

threadInfo, err := net.CreateThread(
    context.Background(),
    threadID,
    core.WithThreadKey(thread.NewServiceKey(threadKey.Service())), // Read key is kept locally
    core.WithLogKey(logPublicKey),                                 // Private key is kept locally
    core.WithNewThreadToken(threadToken))                          // Thread token for identity is needed to verify records

body, err := ipldcbor.WrapObject(map[string]interface{}{
    "foo": "bar",
    "baz": []byte("howdy"),
}, mh.SHA2_256, -1)

// Create the event locally
event, err := cbor.CreateEvent(context.Background(), nil, body, threadKey.Read())

// Create the record locally
record, err := cbor.CreateRecord(context.Background(), nil, cbor.CreateRecordConfig{
	Block:      event,
	Prev:       cid.Undef,              // No previous records because this is the first
	Key:        logPrivateKey,
	PubKey:     myIdentity.GetPublic(),
	ServiceKey: threadKey.Service(),
})

err = net.AddRecord(context.Background(), threadID, logID, record)
```

#### Pulling a thread for new records

Although all known hosts of a particular thread are internally polled for new records (as part of the orchestration protocol), doing so manually can often be useful.

```
err := net.PullThread(context.Background(), info.ID)
```

#### Listening for new records

We can listen for new thread records across all or a subset of known threads.

```
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
records, err := net.Subscribe(ctx, core.WithSubFilter(threadID)) // Only receive new records from this thread

for record := range records {
    // Handle record
}
```

## Developing

The easiest way to develop against `threadsd` is to use the Docker Compose files. The `-dev` flavored file doesn't persist a repo via Docker Volumes, which may be desirable in some cases.

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
