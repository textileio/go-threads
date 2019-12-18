package client

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/api"
	es "github.com/textileio/go-threads/eventstore"
	"github.com/textileio/go-threads/util"
)

const modelName = "Person"

const schema = `{
	"$id": "https://example.com/person.schema.json",
	"$schema": "http://json-schema.org/draft-07/schema#",
	"title": "` + modelName + `",
	"type": "object",
	"required": ["ID"],
	"properties": {
		"ID": {
			"type": "string",
			"description": "The entity's id."
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

var (
	shutdown        func()
	client          *Client
	clientAddr      = "/ip4/127.0.0.1/tcp/9090"
	clientProxyAddr = "/ip4/127.0.0.1/tcp/0"
)

type Person struct {
	ID        string `json:"ID"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Age       int    `json:"age,omitempty"`
}

func TestMain(m *testing.M) {
	_, shutdown = makeServer()
	client = makeClient()
	exitVal := m.Run()
	shutdown()
	os.Exit(exitVal)
}

func TestNewStore(t *testing.T) {
	_, err := client.NewStore()
	if err != nil {
		t.Fatalf("failed to create new store: %v", err)
	}
}

func TestRegisterSchema(t *testing.T) {
	storeID, err := client.NewStore()
	checkErr(t, err)
	err = client.RegisterSchema(storeID, modelName, schema)
	if err != nil {
		t.Fatalf("failed to register schema: %v", err)
	}
}

func TestStart(t *testing.T) {
	storeID, err := client.NewStore()
	checkErr(t, err)
	err = client.RegisterSchema(storeID, modelName, schema)
	checkErr(t, err)
	err = client.Start(storeID)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}
}

func TestStartFromAddress(t *testing.T) {
	storeID, err := client.NewStore()
	checkErr(t, err)
	err = client.RegisterSchema(storeID, modelName, schema)
	checkErr(t, err)

	// TODO: figure out how to test this
	// client.StartFromAddress(storeId, <multiaddress>, <read key>, <follow key>)
}

func TestModelCreate(t *testing.T) {
	storeID, err := client.NewStore()
	checkErr(t, err)
	err = client.RegisterSchema(storeID, modelName, schema)
	checkErr(t, err)
	err = client.Start(storeID)
	checkErr(t, err)

	err = client.ModelCreate(storeID, modelName, createPerson())
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}
}

func TestGetStoreLink(t *testing.T) {
	storeID, err := client.NewStore()
	checkErr(t, err)
	err = client.Start(storeID)
	checkErr(t, err)

	_, err = client.GetStoreLink(storeID)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}
	//@todo: Do proper parsing of the invites
}

func TestModelSave(t *testing.T) {
	storeID, err := client.NewStore()
	checkErr(t, err)
	err = client.RegisterSchema(storeID, modelName, schema)
	checkErr(t, err)
	err = client.Start(storeID)
	checkErr(t, err)

	person := createPerson()

	err = client.ModelCreate(storeID, modelName, person)
	checkErr(t, err)

	person.Age = 30
	err = client.ModelSave(storeID, modelName, person)
	if err != nil {
		t.Fatalf("failed to save model: %v", err)
	}
}

func TestModelDelete(t *testing.T) {
	storeID, err := client.NewStore()
	checkErr(t, err)
	err = client.RegisterSchema(storeID, modelName, schema)
	checkErr(t, err)
	err = client.Start(storeID)
	checkErr(t, err)

	person := createPerson()

	err = client.ModelCreate(storeID, modelName, person)
	checkErr(t, err)

	err = client.ModelDelete(storeID, modelName, person.ID)
	if err != nil {
		t.Fatalf("failed to delete model: %v", err)
	}
}

func TestModelHas(t *testing.T) {
	storeID, err := client.NewStore()
	checkErr(t, err)
	err = client.RegisterSchema(storeID, modelName, schema)
	checkErr(t, err)
	err = client.Start(storeID)
	checkErr(t, err)

	person := createPerson()

	err = client.ModelCreate(storeID, modelName, person)
	checkErr(t, err)

	exists, err := client.ModelHas(storeID, modelName, person.ID)
	if err != nil {
		t.Fatalf("failed to check model has: %v", err)
	}
	if !exists {
		t.Fatal("model should exist but it doesn't")
	}
}

func TestModelFind(t *testing.T) {
	storeID, err := client.NewStore()
	checkErr(t, err)
	err = client.RegisterSchema(storeID, modelName, schema)
	checkErr(t, err)
	err = client.Start(storeID)
	checkErr(t, err)

	person := createPerson()

	err = client.ModelCreate(storeID, modelName, person)
	checkErr(t, err)

	q := es.JSONWhere("lastName").Eq(person.LastName)

	rawResults, err := client.ModelFind(storeID, modelName, q, []*Person{})
	if err != nil {
		t.Fatalf("failed to find: %v", err)
	}
	results := rawResults.([]*Person)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, but got %v", len(results))
	}
	if !reflect.DeepEqual(results[0], person) {
		t.Fatal("model found by query does't equal the original")
	}
}

func TestModelFindByID(t *testing.T) {
	storeID, err := client.NewStore()
	checkErr(t, err)
	err = client.RegisterSchema(storeID, modelName, schema)
	checkErr(t, err)
	err = client.Start(storeID)
	checkErr(t, err)

	person := createPerson()

	err = client.ModelCreate(storeID, modelName, person)
	checkErr(t, err)

	newPerson := &Person{}
	err = client.ModelFindByID(storeID, modelName, person.ID, newPerson)
	if err != nil {
		t.Fatalf("failed to find model by id: %v", err)
	}
	if !reflect.DeepEqual(newPerson, person) {
		t.Fatal("model found by id does't equal the original")
	}
}

func TestReadTransaction(t *testing.T) {
	storeID, err := client.NewStore()
	checkErr(t, err)
	err = client.RegisterSchema(storeID, modelName, schema)
	checkErr(t, err)
	err = client.Start(storeID)
	checkErr(t, err)
	person := createPerson()
	err = client.ModelCreate(storeID, modelName, person)
	checkErr(t, err)

	txn, err := client.ReadTransaction(storeID, modelName)
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
		t.Fatal("txn model found by id does't equal the original")
	}

	q := es.JSONWhere("lastName").Eq(person.LastName)

	rawResults, err := txn.Find(q, []*Person{})
	if err != nil {
		t.Fatalf("failed to find: %v", err)
	}
	results := rawResults.([]*Person)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, but got %v", len(results))
	}
	if !reflect.DeepEqual(results[0], person) {
		t.Fatal("model found by query does't equal the original")
	}
}

func TestWriteTransaction(t *testing.T) {
	storeID, err := client.NewStore()
	checkErr(t, err)
	err = client.RegisterSchema(storeID, modelName, schema)
	checkErr(t, err)
	err = client.Start(storeID)
	checkErr(t, err)
	existingPerson := createPerson()
	err = client.ModelCreate(storeID, modelName, existingPerson)
	checkErr(t, err)

	txn, err := client.WriteTransaction(storeID, modelName)
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

	err = txn.Create(person)
	if err != nil {
		t.Fatalf("failed to create in write txn: %v", err)
	}
	if person.ID == "" {
		t.Fatalf("expected an entity id to be set but it wasn't")
	}

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
		t.Fatalf("txn model found by id does't equal the original")
	}

	q := es.JSONWhere("lastName").Eq(person.LastName)

	rawResults, err := txn.Find(q, []*Person{})
	if err != nil {
		t.Fatalf("failed to find: %v", err)
	}
	results := rawResults.([]*Person)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, but got %v", len(results))
	}
	if !reflect.DeepEqual(results[0], existingPerson) {
		t.Fatal("model found by query does't equal the original")
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
}

func TestListen(t *testing.T) {
	storeID, err := client.NewStore()
	checkErr(t, err)
	err = client.RegisterSchema(storeID, modelName, schema)
	checkErr(t, err)
	err = client.Start(storeID)
	checkErr(t, err)

	person := createPerson()

	err = client.ModelCreate(storeID, modelName, person)
	checkErr(t, err)

	channel, discard := client.Listen(storeID, modelName, person.ID)
	defer discard()

	go func() {
		time.Sleep(1 * time.Second)
		person.Age = 30
		_ = client.ModelSave(storeID, modelName, person)
		person.Age = 40
		_ = client.ModelSave(storeID, modelName, person)
	}()

	val, ok := <-channel
	if !ok {
		t.Fatal("channel no longer active at first event")
	} else {
		if val.err != nil {
			t.Fatalf("failed to receive first listen result: %v", val.err)
		}
		p := &Person{}
		if err := json.Unmarshal(val.data, p); err != nil {
			t.Fatalf("failed to unmarshal listen result: %v", err)
		}
		if p.Age != 30 {
			t.Fatalf("expected listen result age = 30 but got: %v", p.Age)
		}
	}

	val, ok = <-channel
	if !ok {
		t.Fatal("channel no longer active at second event")
	} else {
		if val.err != nil {
			t.Fatalf("failed to receive second listen result: %v", val.err)
		}
		p := &Person{}
		if err := json.Unmarshal(val.data, p); err != nil {
			t.Fatalf("failed to unmarshal listen result: %v", err)
		}
		if p.Age != 40 {
			t.Fatalf("expected listen result age = 40 but got: %v", p.Age)
		}
	}
}

func TestClose(t *testing.T) {
	err := client.Close()
	if err != nil {
		t.Fatalf("failed to close client: %v", err)
	}
}

func makeServer() (*api.Server, func()) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		panic(err)
	}
	ts, err := es.DefaultThreadservice(
		dir,
		es.Debug(true))
	if err != nil {
		panic(err)
	}
	ts.Bootstrap(util.DefaultBoostrapPeers())
	apiAddr, err := ma.NewMultiaddr(clientAddr)
	if err != nil {
		panic(err)
	}
	apiProxyAddr, err := ma.NewMultiaddr(clientProxyAddr)
	if err != nil {
		panic(err)
	}
	server, err := api.NewServer(context.Background(), ts, api.Config{
		RepoPath:  dir,
		Addr:      apiAddr,
		ProxyAddr: apiProxyAddr,
		Debug:     true,
	})
	if err != nil {
		panic(err)
	}
	return server, func() {
		server.Close()
		if err := ts.Close(); err != nil {
			panic(err)
		}
		_ = os.RemoveAll(dir)
	}
}

func makeClient() *Client {
	addr, err := ma.NewMultiaddr(clientAddr)
	if err != nil {
		panic(err)
	}
	client, err := NewClient(addr)
	if err != nil {
		panic(err)
	}
	return client
}

func checkErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func createPerson() *Person {
	return &Person{
		ID:        "",
		FirstName: "Adam",
		LastName:  "Doe",
		Age:       21,
	}
}
