# Store
`Store` is a json-document database backed by Threads V2.

This document describes its public API, and its internal design/architecture. 
Internal understanding isn't necessary to use `Store`, but will help understand 
how things are wired. Creating a `Store` under the default configuration will 
automatically build everything required to work.

Currently, a `Store` is backed by a single Thread. In the future, this can 
change making the `Store` map different `Model`s to different Threads, or 
any other combination.

## Usage
ToDo: Describe public API here.

## Internal Design
In this section, there is a high-level overview of internal `Store` design.

### Diagram
The following diagram try to express all components and their relationships and 
communications:

![Design](design.png)

The above diagram depicts the different components inside a `Store`. Also, it 
travels through their relationships caused by a transaction commit. The inverse 
path, caused by a new event detected in other peer log in the thread, is somewhat 
similar but in the other direction.

Arrows aren't always synchronous calls, but also channel notifications and other 
mediums in order to inverse dependency between components. Arrows are conceptual 
communications.

#### Models
Models are part of `Store` public-api.
Main responsibility: store instances of a user-defined schema.

Models are json-schemas that describe instance types of the `Store`. They provide 
the public API for creating, deleting, updating, and querying instances within 
this model. They also provide read/write transactions which have _serializable isolation_ 
within the `Store` scope.



#### EventCodec
This is an internal component not available in the public API.
Main responibility: Transform actions done in instances within a transaction, to
`Event`s (as bytes in the `Event` payload)

`EventCodec` is an abstraction used to transform actions applied to model instances in 
a transaction, to `Event`. An `Event` is conceptually something that happened in the 
`Store`.

For example, if within a model `WriteTxn()`, a new instance is created and another 
was updated, these two action will be sent to the `EventCodec` to transform them in 
`Event`s. These `Event` have a byte payload with the encoded transformation. Currently, 
the only implementation of `EventCodec` is a `jsonpatcher`, which transforms these actions 
in json-merge/patches, and store them as payloads in events. 

These generated `Events` will be passed to the underlying `Threadservice` to add in 
the peer own log in the thread associated with the `Store`. Likewise, `EventCodec` 
also do the inverse transformation.  Given a `Event`, it transforms its byte payload 
into actions that will be applied in the model.

The `EventCodec` abstraction allows an extensibility point. If instead of a json-patcher 
we want to encode model changes as full instance snapshots (i.e: instead of generating 
the json-patch, let generate the full instance data), we could provide another implementation 
of the `EventCodec` to use in the Store.

Similarly, more advanced encodings of JSON-Document changes can be implemented as `EventCodec` 
such as JSON-Documents-Delta-CRDTs, or a hybrid json-patch with logical clocks.

In summary, `EventCodec` encodes and decodes model actions to byte payloads. These payloads 
will ultimately be stored in logs in a `Thread`, via the `Dispatcher` and eventually `Threadservice`.


#### Dispatcher
This is an internal component not available in the public API.
Main responsibility: Every new `Event`, generated internally through transactions or externally 
via discovering new `Event` from peer logs, must go through the dispatcher which will dispatch 
it to all `Model`s to perform the `Reduce()` action (change their state).

Every `Event` generated in the `Store` is sent to a `Dispatcher` when write transactions 
are committed. The dispatcher is responsible for broadcasting these events to all Reducers. 
A reducer is a party which is interested in knowing about `Store` events. In particular for 
`Store`, `Model` are reducers that apply `Event` changes using `EventCodecs`.

For example, if a particular instance is updated in a `Model`, these corresponding actions 
will be encoded as `Event` by the `EventCodec` as mentioned in the last section. These `Events` 
will be dispatched to the `Dispatcher`, which will broadcast them to all registered Reducers (which 
include the original `Model` that generated them). The model will apply those changes to its 
internal state (so to apply the originally intended changes of the instance).

The implications of this design implies that all `Model` state change can only happen when 
the `Dispatcher` sends events to reducers (`Model`s). A `Model` can't distinguish between 
received `Events` generated locally or externally. External events are the results of `Threadservice` 
dispatching new events to the `Dispatcher`; which means that new `Event`s where detected in 
other peer logs of the same Thread.

#### Datastore
This is an internal component not available in the public API.
Main responsibility: Delivering durable persistence for data.

`Datastore` is the underlying persistence of `Model` instances and `Dispatcher` raw `Event` 
information.

#### Local Event Bus
This is an internal component not available in the public API.
Main responsibility: Deliver locally generated `Event` to other internal components. Currently, 
only to `SingleThreadAdapter`. In the future could be used too feed other task needed to run 
when *local changes* are created.

`LocalEventBus` Is a `Broadcaster` which notifies interested parties about locally generated `Events`. 
Its only user is a `StoreThreadAdapter` (which will be explained below).

#### Store State-change broadcaster
This is part of the public-api. 
Main responsibility: Notify external actors that the `Store` changed its state. S

Broadcaster useful for end-users that wants to be notified when any `Model` in 
the `Store` has reduced a new local/remote `Event`. Saying it differently, when some data changed.

### SingleThreadAdapter (StoreThreadAdapter)
This is an internal component not available in the public API.
Main responsibility: Map new `Event` generated in `Store`, to a Threads architecture. Currently, 
one-to-one mapping, but could be different.

`SingleThreadAdapter` is a component that maps `Events` in the `Store` sense, with `Events` in the 
Thread sense. 
Every time a new local `Event` is generated in the `Store` (by a `EventCodec` dispatched to `Dispatcher`), 
will eventually reach the `SingleThreadAdapter`. At this point, `SingleThreadAdapter` will transform 
it in an `Event` that will be appended in the local peer own log in the Thread (using `Threadservice`).

Similarly, when `Threadservice` detects new events in other peer logs, it will dispatch them to 
`SingleThreadAdapter`. Then, it will transform it into a Store `Event` that will be dispatched to 
`Dispatcher` and ultimately reach the `Model`, which will apply the change to their state (if this Event 
corresponds to them).

As said initially, currently, the `Store` is only mapped to a single Thread. But in other implementations 
could decide to map different types of `Events` to different `Threads`. `SingleThreadAdapter` main decision 
is where to send `Store` events to one or more Threads.

Note: `SingleThreadAdapter` is a biased name towards an implementation of mapping to a unique thread for 
the `Store`. A better name for this component would be `StoreThreadAdapter`.

### Threadservice
This component is part of the public-api so that it can be accessed.
Main responsibility: Is the `Store` interface with Threads layer.

`Threadservice` is the bidirectional communication interface between a Thread and the `Store`. 
`Threadservice` will only interact with `StoreThreadAdapter`.

