package client_test

import (
	"context"
	crand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/phayes/freeport"
	"github.com/textileio/go-threads/api"
	. "github.com/textileio/go-threads/api/client"
	pb "github.com/textileio/go-threads/api/pb"
	"github.com/textileio/go-threads/common"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	"github.com/textileio/go-threads/util"
	"google.golang.org/grpc"
)

func TestClient_GetToken(t *testing.T) {
	t.Parallel()
	client, done := setup(t)
	defer done()

	identity := createIdentity(t)

	t.Run("test get token", func(t *testing.T) {
		tok, err := client.GetToken(context.Background(), identity)
		if err != nil {
			t.Fatalf("failed to get token: %v", err)
		}
		if tok == "" {
			t.Fatal("emtpy token")
		}
	})
}

func TestClient_NewDB(t *testing.T) {
	t.Parallel()
	client, done := setup(t)
	defer done()

	t.Run("test new db", func(t *testing.T) {
		if err := client.NewDB(context.Background(), thread.NewIDV1(thread.Raw, 32)); err != nil {
			t.Fatalf("failed to create new db: %v", err)
		}
	})
}

func TestClient_NewDBFromAddr(t *testing.T) {
	t.Parallel()
	client1, done1 := setup(t)
	defer done1()
	client2, done2 := setup(t)
	defer done2()

	id := thread.NewIDV1(thread.Raw, 32)
	err := client1.NewDB(context.Background(), id)
	checkErr(t, err)
	info, err := client1.GetDBInfo(context.Background(), id)
	checkErr(t, err)

	t.Run("test new db from address", func(t *testing.T) {
		if err = client2.NewDBFromAddr(context.Background(), info.Addrs[0], info.Key); err != nil {
			t.Fatalf("failed to create new db from address: %v", err)
		}
	})
}

func TestClient_ListDBs(t *testing.T) {
	t.Parallel()
	client, done := setup(t)
	defer done()

	t.Run("test list dbs", func(t *testing.T) {
		id1 := thread.NewIDV1(thread.Raw, 32)
		name1 := "db1"
		err := client.NewDB(context.Background(), id1, db.WithNewManagedName(name1))
		checkErr(t, err)
		id2 := thread.NewIDV1(thread.Raw, 32)
		name2 := "db2"
		err = client.NewDB(context.Background(), id2, db.WithNewManagedName(name2))
		checkErr(t, err)

		list, err := client.ListDBs(context.Background())
		if err != nil {
			t.Fatalf("failed to list dbs: %v", err)
		}
		if len(list) != 2 {
			t.Fatalf("expected 2 dbs, but got %v", len(list))
		}
		if list[id1].Name != name1 {
			t.Fatalf("expected name to be %s, but got %s", name1, list[id1].Name)
		}
		if list[id2].Name != name2 {
			t.Fatalf("expected name to be %s, but got %s", name2, list[id2].Name)
		}
	})
}

func TestClient_GetDBInfo(t *testing.T) {
	t.Parallel()
	client, done := setup(t)
	defer done()

	t.Run("test get db info", func(t *testing.T) {
		id := thread.NewIDV1(thread.Raw, 32)
		err := client.NewDB(context.Background(), id)
		checkErr(t, err)

		info, err := client.GetDBInfo(context.Background(), id)
		if err != nil {
			t.Fatalf("failed to get db info: %v", err)
		}
		if !info.Key.Defined() {
			t.Fatal("got undefined db key")
		}
		if len(info.Addrs) == 0 {
			t.Fatal("got empty addresses")
		}
	})
}

func TestClient_DeleteDB(t *testing.T) {
	t.Parallel()
	client, done := setup(t)
	defer done()

	t.Run("test delete db", func(t *testing.T) {
		id := thread.NewIDV1(thread.Raw, 32)
		err := client.NewDB(context.Background(), id)
		checkErr(t, err)

		if err = client.DeleteDB(context.Background(), id); err != nil {
			t.Fatalf("failed to delete db: %v", err)
		}
		if _, err := client.GetDBInfo(context.Background(), id); err == nil {
			t.Fatal("deleted db still exists")
		}
	})
}

func TestClient_NewCollection(t *testing.T) {
	t.Parallel()
	client, done := setup(t)
	defer done()

	t.Run("test new collection", func(t *testing.T) {
		id := thread.NewIDV1(thread.Raw, 32)
		err := client.NewDB(context.Background(), id)
		checkErr(t, err)
		err = client.NewCollection(context.Background(), id, db.CollectionConfig{Name: collectionName, Schema: util.SchemaFromSchemaString(schema)})
		if err != nil {
			t.Fatalf("failed to add new collection: %v", err)
		}
	})
}

func TestClient_UpdateCollection(t *testing.T) {
	t.Parallel()
	client, done := setup(t)
	defer done()

	id := thread.NewIDV1(thread.Raw, 32)
	err := client.NewDB(context.Background(), id)
	checkErr(t, err)
	err = client.NewCollection(context.Background(), id, db.CollectionConfig{Name: collectionName, Schema: util.SchemaFromSchemaString(schema)})
	checkErr(t, err)

	t.Run("test update collection", func(t *testing.T) {
		err = client.UpdateCollection(context.Background(), id, db.CollectionConfig{
			Name:   collectionName,
			Schema: util.SchemaFromSchemaString(schema2),
			Indexes: []db.Index{{
				Path:   "age",
				Unique: false,
			}},
		})
		if err != nil {
			t.Fatalf("failed to update collection: %v", err)
		}
		_, err = client.Create(context.Background(), id, collectionName, Instances{createPerson2()})
		checkErr(t, err)
	})
}

func TestClient_DeleteCollection(t *testing.T) {
	t.Parallel()
	client, done := setup(t)
	defer done()

	t.Run("test delete collection", func(t *testing.T) {
		id := thread.NewIDV1(thread.Raw, 32)
		err := client.NewDB(context.Background(), id)
		checkErr(t, err)
		err = client.NewCollection(context.Background(), id, db.CollectionConfig{Name: collectionName, Schema: util.SchemaFromSchemaString(schema)})
		checkErr(t, err)

		err = client.DeleteCollection(context.Background(), id, collectionName)
		if err != nil {
			t.Fatalf("failed to delete collection: %v", err)
		}
		_, err = client.Create(context.Background(), id, collectionName, Instances{createPerson()})
		if err == nil {
			t.Fatal("failed to delete collection")
		}
	})
}

func TestClient_GetCollectionIndexes(t *testing.T) {
	t.Parallel()
	client, done := setup(t)
	defer done()

	t.Run("test get collection indexes", func(t *testing.T) {
		id := thread.NewIDV1(thread.Raw, 32)
		err := client.NewDB(context.Background(), id)
		checkErr(t, err)
		err = client.NewCollection(context.Background(), id, db.CollectionConfig{
			Name:   collectionName,
			Schema: util.SchemaFromSchemaString(schema),
			Indexes: []db.Index{{
				Path:   "lastName",
				Unique: true,
			}},
		})
		checkErr(t, err)
		indexes, err := client.GetCollectionIndexes(context.Background(), id, collectionName)
		checkErr(t, err)
		if len(indexes) != 2 {
			t.Fatalf("expected 2 indexes, but got %v", len(indexes))
		}
	})
}

func TestClient_Create(t *testing.T) {
	t.Parallel()
	client, done := setup(t)
	defer done()

	t.Run("test collection create", func(t *testing.T) {
		id := thread.NewIDV1(thread.Raw, 32)
		err := client.NewDB(context.Background(), id)
		checkErr(t, err)
		err = client.NewCollection(context.Background(), id, db.CollectionConfig{Name: collectionName, Schema: util.SchemaFromSchemaString(schema)})
		checkErr(t, err)

		_, err = client.Create(context.Background(), id, collectionName, Instances{createPerson()})
		if err != nil {
			t.Fatalf("failed to create instance: %v", err)
		}
	})

	t.Run("test collection create with missing id", func(t *testing.T) {
		id := thread.NewIDV1(thread.Raw, 32)
		err := client.NewDB(context.Background(), id)
		checkErr(t, err)
		err = client.NewCollection(context.Background(), id, db.CollectionConfig{Name: collectionName, Schema: util.SchemaFromSchemaString(schema)})
		checkErr(t, err)

		ids, err := client.Create(context.Background(), id, collectionName, Instances{&PersonWithoutID{
			FirstName: "Adam",
			LastName:  "Doe",
			Age:       21,
		}})
		if err != nil {
			t.Fatalf("failed to create instance: %v", err)
		}
		if len(ids) != 1 {
			t.Fatal("expected a new id, got none")
		}
	})
}

func TestClient_Save(t *testing.T) {
	t.Parallel()
	client, done := setup(t)
	defer done()

	t.Run("test collection save", func(t *testing.T) {
		id := thread.NewIDV1(thread.Raw, 32)
		err := client.NewDB(context.Background(), id)
		checkErr(t, err)
		err = client.NewCollection(context.Background(), id, db.CollectionConfig{Name: collectionName, Schema: util.SchemaFromSchemaString(schema)})
		checkErr(t, err)

		person := createPerson()

		ids, err := client.Create(context.Background(), id, collectionName, Instances{person})
		checkErr(t, err)

		person.ID = ids[0]
		person.Age = 30
		err = client.Save(context.Background(), id, collectionName, Instances{person})
		if err != nil {
			t.Fatalf("failed to save instance: %v", err)
		}
	})
}

func TestClient_Delete(t *testing.T) {
	t.Parallel()
	client, done := setup(t)
	defer done()

	t.Run("test collection delete", func(t *testing.T) {
		id := thread.NewIDV1(thread.Raw, 32)
		err := client.NewDB(context.Background(), id)
		checkErr(t, err)
		err = client.NewCollection(context.Background(), id, db.CollectionConfig{Name: collectionName, Schema: util.SchemaFromSchemaString(schema)})
		checkErr(t, err)

		person := createPerson()

		ids, err := client.Create(context.Background(), id, collectionName, Instances{person})
		checkErr(t, err)

		person.ID = ids[0]

		err = client.Delete(context.Background(), id, collectionName, []string{person.ID})
		if err != nil {
			t.Fatalf("failed to delete instance: %v", err)
		}
	})
}

func TestClient_Has(t *testing.T) {
	t.Parallel()
	client, done := setup(t)
	defer done()

	t.Run("test collection has", func(t *testing.T) {
		id := thread.NewIDV1(thread.Raw, 32)
		err := client.NewDB(context.Background(), id)
		checkErr(t, err)
		err = client.NewCollection(context.Background(), id, db.CollectionConfig{Name: collectionName, Schema: util.SchemaFromSchemaString(schema)})
		checkErr(t, err)

		person := createPerson()

		ids, err := client.Create(context.Background(), id, collectionName, Instances{person})
		checkErr(t, err)

		person.ID = ids[0]

		exists, err := client.Has(context.Background(), id, collectionName, []string{person.ID})
		if err != nil {
			t.Fatalf("failed to check collection has: %v", err)
		}
		if !exists {
			t.Fatal("collection should exist but it doesn't")
		}
	})
}

func TestClient_Find(t *testing.T) {
	t.Parallel()
	client, done := setup(t)
	defer done()

	t.Run("test collection find", func(t *testing.T) {
		id := thread.NewIDV1(thread.Raw, 32)
		err := client.NewDB(context.Background(), id)
		checkErr(t, err)
		err = client.NewCollection(context.Background(), id, db.CollectionConfig{Name: collectionName, Schema: util.SchemaFromSchemaString(schema)})
		checkErr(t, err)

		person := createPerson()

		ids, err := client.Create(context.Background(), id, collectionName, Instances{person})
		checkErr(t, err)

		person.ID = ids[0]

		q := db.Where("lastName").Eq(person.LastName)

		rawResults, err := client.Find(context.Background(), id, collectionName, q, &Person{})
		if err != nil {
			t.Fatalf("failed to find: %v", err)
		}
		results := rawResults.([]*Person)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, but got %v", len(results))
		}
		if !reflect.DeepEqual(results[0], person) {
			t.Fatal("collection found by query does't equal the original")
		}
	})
}

func TestClient_FindWithIndex(t *testing.T) {
	t.Parallel()
	client, done := setup(t)
	defer done()
	t.Run("test collection find", func(t *testing.T) {
		id := thread.NewIDV1(thread.Raw, 32)
		err := client.NewDB(context.Background(), id)
		checkErr(t, err)
		err = client.NewCollection(context.Background(), id, db.CollectionConfig{
			Name:   collectionName,
			Schema: util.SchemaFromSchemaString(schema),
			Indexes: []db.Index{{
				Path:   "lastName",
				Unique: true,
			}},
		})
		checkErr(t, err)

		person := createPerson()

		ids, err := client.Create(context.Background(), id, collectionName, Instances{person})
		checkErr(t, err)

		person.ID = ids[0]

		q := db.Where("lastName").Eq(person.LastName).UseIndex("lastName")

		rawResults, err := client.Find(context.Background(), id, collectionName, q, &Person{})
		if err != nil {
			t.Fatalf("failed to find: %v", err)
		}
		results := rawResults.([]*Person)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, but got %v", len(results))
		}
		if !reflect.DeepEqual(results[0], person) {
			t.Fatal("collection found by query does't equal the original")
		}
	})
}

func TestClient_FindByID(t *testing.T) {
	t.Parallel()
	client, done := setup(t)
	defer done()

	t.Run("test collection find by ID", func(t *testing.T) {
		id := thread.NewIDV1(thread.Raw, 32)
		err := client.NewDB(context.Background(), id)
		checkErr(t, err)
		err = client.NewCollection(context.Background(), id, db.CollectionConfig{Name: collectionName, Schema: util.SchemaFromSchemaString(schema)})
		checkErr(t, err)

		person := createPerson()

		ids, err := client.Create(context.Background(), id, collectionName, Instances{person})
		checkErr(t, err)

		person.ID = ids[0]

		newPerson := &Person{}
		err = client.FindByID(context.Background(), id, collectionName, person.ID, newPerson)
		if err != nil {
			t.Fatalf("failed to find collection by id: %v", err)
		}
		if !reflect.DeepEqual(newPerson, person) {
			t.Fatal("collection found by id does't equal the original")
		}
	})
}

func TestClient_ReadTransaction(t *testing.T) {
	t.Parallel()
	client, done := setup(t)
	defer done()

	t.Run("test read transaction", func(t *testing.T) {
		id := thread.NewIDV1(thread.Raw, 32)
		err := client.NewDB(context.Background(), id)
		checkErr(t, err)
		err = client.NewCollection(context.Background(), id, db.CollectionConfig{Name: collectionName, Schema: util.SchemaFromSchemaString(schema)})
		checkErr(t, err)

		person := createPerson()
		ids, err := client.Create(context.Background(), id, collectionName, Instances{person})
		checkErr(t, err)

		person.ID = ids[0]

		txn, err := client.ReadTransaction(context.Background(), id, collectionName)
		if err != nil {
			t.Fatalf("failed to create read txn: %v", err)
		}

		end, err := txn.Start()
		defer func() {
			err = end()
			if err != nil {
				t.Fatalf("failed to end txn: %v", err)
			}
		}()
		if err != nil {
			t.Fatalf("failed to start read txn: %v", err)
		}

		has, err := txn.Has(person.ID)
		if err != nil {
			t.Fatalf("failed to read txn has: %v", err)
		}
		if !has {
			t.Fatal("expected has to be true but it wasn't")
		}

		foundPerson := &Person{}
		err = txn.FindByID(person.ID, foundPerson)
		if err != nil {
			t.Fatalf("failed to txn find by id: %v", err)
		}
		if !reflect.DeepEqual(foundPerson, person) {
			t.Fatal("txn collection found by id does't equal the original")
		}

		q := db.Where("lastName").Eq(person.LastName)

		rawResults, err := txn.Find(q, &Person{})
		if err != nil {
			t.Fatalf("failed to find: %v", err)
		}
		results := rawResults.([]*Person)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, but got %v", len(results))
		}
		if !reflect.DeepEqual(results[0], person) {
			t.Fatal("collection found by query does't equal the original")
		}
	})
}

func TestClient_WriteTransaction(t *testing.T) {
	t.Parallel()
	client, done := setup(t)
	defer done()

	t.Run("test write transaction", func(t *testing.T) {
		id := thread.NewIDV1(thread.Raw, 32)
		err := client.NewDB(context.Background(), id)
		checkErr(t, err)
		err = client.NewCollection(context.Background(), id, db.CollectionConfig{Name: collectionName, Schema: util.SchemaFromSchemaString(schema)})
		checkErr(t, err)

		existingPerson := createPerson()

		ids, err := client.Create(context.Background(), id, collectionName, Instances{existingPerson})
		checkErr(t, err)

		existingPerson.ID = ids[0]

		txn, err := client.WriteTransaction(context.Background(), id, collectionName)
		if err != nil {
			t.Fatalf("failed to create write txn: %v", err)
		}

		end, err := txn.Start()
		defer func() {
			err = end()
			if err != nil {
				t.Fatalf("failed to end txn: %v", err)
			}
		}()
		if err != nil {
			t.Fatalf("failed to start write txn: %v", err)
		}

		person := createPerson()

		ids, err = txn.Create(person)
		if err != nil {
			t.Fatalf("failed to create in write txn: %v", err)
		}

		person.ID = ids[0]

		has, err := txn.Has(existingPerson.ID)
		if err != nil {
			t.Fatalf("failed to write txn has: %v", err)
		}
		if !has {
			t.Fatalf("expected has to be true but it wasn't")
		}

		foundExistingPerson := &Person{}
		err = txn.FindByID(existingPerson.ID, foundExistingPerson)
		if err != nil {
			t.Fatalf("failed to txn find by id: %v", err)
		}
		if !reflect.DeepEqual(foundExistingPerson, existingPerson) {
			t.Fatalf("txn collection found by id does't equal the original")
		}

		q := db.Where("lastName").Eq(person.LastName)

		rawResults, err := txn.Find(q, &Person{})
		if err != nil {
			t.Fatalf("failed to find: %v", err)
		}
		results := rawResults.([]*Person)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, but got %v", len(results))
		}
		if !reflect.DeepEqual(results[0], existingPerson) {
			t.Fatal("collection found by query does't equal the original")
		}

		existingPerson.Age = 99
		err = txn.Save(existingPerson)
		if err != nil {
			t.Fatalf("failed to save in write txn: %v", err)
		}

		err = txn.Delete(existingPerson.ID)
		if err != nil {
			t.Fatalf("failed to delete in write txn: %v", err)
		}
	})
}

func TestClient_Listen(t *testing.T) {
	t.Parallel()
	client, done := setup(t)
	defer done()

	t.Run("test listen", func(t *testing.T) {
		id := thread.NewIDV1(thread.Raw, 32)
		err := client.NewDB(context.Background(), id)
		checkErr(t, err)
		err = client.NewCollection(context.Background(), id, db.CollectionConfig{Name: collectionName, Schema: util.SchemaFromSchemaString(schema)})
		checkErr(t, err)

		person := createPerson()

		ids, err := client.Create(context.Background(), id, collectionName, Instances{person})
		checkErr(t, err)

		person.ID = ids[0]

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		opt := ListenOption{
			Collection: collectionName,
			InstanceID: person.ID,
		}
		channel, err := client.Listen(ctx, id, []ListenOption{opt})
		if err != nil {
			t.Fatalf("failed to call listen: %v", err)
		}

		go func() {
			time.Sleep(time.Second)
			person.Age = 30
			_ = client.Save(context.Background(), id, collectionName, Instances{person})
			person.Age = 40
			_ = client.Save(context.Background(), id, collectionName, Instances{person})
		}()

		val, ok := <-channel
		if !ok {
			t.Fatal("channel no longer active at first event")
		} else {
			if val.Err != nil {
				t.Fatalf("failed to receive first listen result: %v", val.Err)
			}
			p := &Person{}
			if err := json.Unmarshal(val.Action.Instance, p); err != nil {
				t.Fatalf("failed to unmarshal listen result: %v", err)
			}
			if p.Age != 30 {
				t.Fatalf("expected listen result age = 30 but got: %v", p.Age)
			}
			if val.Action.InstanceID != person.ID {
				t.Fatalf("expected listen result id = %v but got: %v", person.ID, val.Action.InstanceID)
			}
		}

		val, ok = <-channel
		if !ok {
			t.Fatal("channel no longer active at second event")
		} else {
			if val.Err != nil {
				t.Fatalf("failed to receive second listen result: %v", val.Err)
			}
			p := &Person{}
			if err := json.Unmarshal(val.Action.Instance, p); err != nil {
				t.Fatalf("failed to unmarshal listen result: %v", err)
			}
			if p.Age != 40 {
				t.Fatalf("expected listen result age = 40 but got: %v", p.Age)
			}
			if val.Action.InstanceID != person.ID {
				t.Fatalf("expected listen result id = %v but got: %v", person.ID, val.Action.InstanceID)
			}
		}
	})
}

func TestClient_Close(t *testing.T) {
	t.Parallel()
	addr, shutdown := makeServer(t)
	defer shutdown()
	target, err := util.TCPAddrFromMultiAddr(addr)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(target, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test close", func(t *testing.T) {
		if err := client.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

func setup(t *testing.T) (*Client, func()) {
	addr, shutdown := makeServer(t)
	target, err := util.TCPAddrFromMultiAddr(addr)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(target, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}

	return client, func() {
		shutdown()
		_ = client.Close()
	}
}

func makeServer(t *testing.T) (ma.Multiaddr, func()) {
	time.Sleep(time.Second * time.Duration(rand.Intn(5)))
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	n, err := common.DefaultNetwork(dir, common.WithNetDebug(true), common.WithNetHostAddr(util.FreeLocalAddr()))
	if err != nil {
		t.Fatal(err)
	}
	n.Bootstrap(util.DefaultBoostrapPeers())
	service, err := api.NewService(n, api.Config{
		RepoPath: dir,
		Debug:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	port, err := freeport.GetFreePort()
	if err != nil {
		t.Fatal(err)
	}
	addr := util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port))
	target, err := util.TCPAddrFromMultiAddr(addr)
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	listener, err := net.Listen("tcp", target)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		pb.RegisterAPIServer(server, service)
		if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Fatalf("serve error: %v", err)
		}
	}()

	return addr, func() {
		time.Sleep(time.Second) // Give threads a chance to finish work
		server.GracefulStop()
		if err := n.Close(); err != nil {
			t.Fatal(err)
		}
		_ = os.RemoveAll(dir)
	}
}

func checkErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func createIdentity(t *testing.T) thread.Identity {
	sk, _, err := crypto.GenerateEd25519Key(crand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return thread.NewLibp2pIdentity(sk)
}

func createPerson() *Person {
	return &Person{
		FirstName: "Adam",
		LastName:  "Doe",
		Age:       21,
	}
}

func createPerson2() *Person2 {
	return &Person2{
		FullName: "Adam Doe",
		Age:      21,
	}
}

const (
	collectionName = "Person"

	schema = `{
		"$id": "https://example.com/person.schema.json",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"title": "` + collectionName + `",
		"type": "object",
		"properties": {
			"_id": {
				"type": "string",
				"description": "The instance's id."
			},
			"firstName": {
				"type": "string",
				"description": "The person's first name."
			},
			"lastName": {
				"type": "string",
				"description": "The person's last name."
			},
			"age": {
				"description": "Age in years which must be equal to or greater than zero.",
				"type": "integer",
				"minimum": 0
			}
		}
	}`

	schema2 = `{
		"$id": "https://example.com/person.schema.json",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"title": "` + collectionName + `",
		"type": "object",
		"properties": {
			"_id": {
				"type": "string",
				"description": "The instance's id."
			},
			"fullName": {
				"type": "string",
				"description": "The person's full name."
			},
			"age": {
				"description": "Age in years which must be equal to or greater than zero.",
				"type": "integer",
				"minimum": 0
			}
		}
	}`
)

type Person struct {
	ID        string `json:"_id"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Age       int    `json:"age,omitempty"`
}

type Person2 struct {
	ID       string `json:"_id"`
	FullName string `json:"fullName,omitempty"`
	Age      int    `json:"age,omitempty"`
}

type PersonWithoutID struct {
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Age       int    `json:"age,omitempty"`
}
