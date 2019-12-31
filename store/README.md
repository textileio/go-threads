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
path, caused by a new event detected in other peer log in the thread, is 
somewhat similar but in the other direction.

Arrows aren't always synchronous calls, but also channel notifications and 
other mediums in order to inverse dependency between components. Arrows are 
conceptual communications.

#### Models
Models are part of `Store` public-api.
Main responsibility: store instances of a user-defined schema.

Models are json-schemas that describe instance types of the `Store`. They 
provide the public API for creating, deleting, updating, and querying instances 
within this model. They also provide read/write transactions which have 
_serializable isolation_ within the `Store` scope.


#### EventCodec
This is an internal component not available in the public API.
Main responsibility: Transform and apply and encode/decode transaction actions.

`EventCodec` is an abstraction used to:
- Transform actions made in a txn, to an array of `store.Event` that will be 
dispatcher to be reduced.
- Encode actions made in a txn to a `format.Node` which will serve as 
the next building block for the appended Record in the local peer log.
- The reverse of last point, when receiving external actions to allow to be 
dispatched.

For example, if within a model `WriteTxn()`, a new instance is created and 
other was updated, these two action will be sent to the `EventCodec` to 
transform them in `Event`s. These `Event` have a byte payload with the encoded 
transformation. Currently, the only implementation of `EventCodec` is a 
`jsonpatcher`, which transforms these actions in json-merge/patches, and store 
them as payloads in events. 

These events are also aggregated in a returned `format.Node`, which is the 
compatible/analogous information to be used by `Threadservice` to add in 
the peer own log in the thread associated with the `Store`. Likewise, 
`EventCodec` also do the inverse transformation.  Given a `format.Node`, it 
transforms its byte payload into actions that will be reduced in the store.

The `EventCodec` abstraction allows an extensibility point. If instead of a 
json-patcher we want to encode model changes as full instance snapshots 
(i.e: instead of generating the json-patch, let generate the full instance 
data), we could provide another implementation of the `EventCodec` to use in 
the Store.

Similarly, more advanced encodings of JSON-Document changes can be implemented 
as `EventCodec` such as JSON-Documents-Delta-CRDTs, or a hybrid json-patch 
with logical clocks.


#### Dispatcher
This is an internal component not available in the public API.
Main responsibility: Source of truth regarding known `store.Event`s for the 
`Store`. Will notify registered parties to let them know about new ones..

Every `Event` generated in the `Store` is sent to a `Dispatcher` when write 
transactions are committed. The dispatcher is responsible for broadcasting 
these events to all registered Reducers. A reducer is a party which is 
interested in knowing about `Store` events. Currently, the only reducer is the 
`Store` itself.

For example, if a particular instance is updated in a `Model`, these 
corresponding actions will be encoded as `Event` by the `EventCodec` as 
mentioned in the last section. These `Events` will be dispatched to the 
`Dispatcher`, which will:
- Store the new event in durable storage. If the txn made multiple changes, 
this is done transactionally.
- Broadcast them to all registered Reducers (which currently is only `Store`). 
Reducers will apply those changes for their own interests.

The implications of this design imply that real `Store` state changes can 
only happen when the `Dispatcher` broadcast new `store.Event`s. 
A `Reducer` can't distinguish between `Events` generated locally or externally. 
External events are the results of `Threadservice` sending new events to the 
`Dispatcher`, which means that new `Event`s where detected in other peer logs 
of the same Thread.

#### Datastore
This is an internal component not available in the public API.
Main responsibility: Delivering durable persistence for data.

`Datastore` is the underlying persistence of ``Model`` instances and 
`Dispatcher` raw `Event` information. In both cases, their interface is a 
`datastore.TxnDatastore` to have txn guarantees.

#### Local Event Bus
This is an internal component not available in the public API.
Main responsibility: Deliver `format.Node` encoded information of changes 
done in local commited transactions. Currently, only to `SingleThreadAdapter` 
is listening to this bus. 


#### Store Listener
This is part of the public-api. 
Main responsibility: Notify external actors that the `Store` changed its state, 
with details about the change: in which model, what action (Create, Save, 
Delete), and wich EntityID.

Listeners are useful for clients that want to be notified about changes in the 
`Store`. Recall that `Store` state can change by external events, such as 
receiving external changes from other peers sharing the `Store`.

The client can configure which kind of events wants to be notified. Can add 
any number of criterias; if more than one criteria is used they will be 
interpreted as _OR_ conditions.
A criteria contains the following information:
- Which model to listen changes
- What action is done (Create, Save, Delete)
- Which EntitiID

Any of the above three attributes can be set empty. For example, we can listen 
to all changes of all entities in a model if only the first attribute is set 
and the other two are left empty/default.

### StoreThreadAdapter (SingleThreadAdapter, unique implementation)
This is an internal component not available in the public API.
Main responsibility: Responsible to be the two-way communication between 
`Store` and `Threads`.

Every time a new local `format.Node` is generated in the `Store` due to a 
write transaction commit, the `StoreThreadAdapter` will notify `Threadservice` 
that a new `Record` should be added to the local peer log.

Similarly, when `Threadservice` detects new `Record`s in other peer logs, it 
will dispatch them to `SingleThreadAdapter`. Then, it will transform it into a 
Store `Event`s that will be dispatched to `Dispatcher` and ultimately will 
be reduced to impact `Store` state.

As said initially, currently, the `Store` is only mapped to a single Thread. 
But is possible to decide a different map, where a `Store` might be backed by 
more than one thread or any other schema. This is the component that should 
be taking this decisions.


### Threadservice
This component is part of the public-api so that it can be accessed.
Main responsibility: Is the `Store` interface with Threads layer.

`Threadservice` is the bidirectional communication interface to the underlying 
Thread backing the `Store`. It only interacts with `StoreThreadAdapter`

