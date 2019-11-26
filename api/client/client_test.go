package client

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/textileio/go-textile-threads/api"
	es "github.com/textileio/go-textile-threads/eventstore"
	"github.com/textileio/go-textile-threads/util"
)

const schema = `{
	"$id": "https://example.com/person.schema.json",
	"$schema": "http://json-schema.org/draft-07/schema#",
	"title": "Person",
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

type Person struct {
	ID        string
	firstName string
	lastName  string
	age       int
}

var adam = &Person{
	ID:        "",
	firstName: "Adam",
	lastName:  "Doe",
	age:       21,
}

var eve = &Person{
	ID:        "",
	firstName: "Eve",
	lastName:  "Doe",
	age:       21,
}

func TestNewStore(t *testing.T) {
	_, clean := server(t)
	defer clean()
	client := client(t)

	_, err := client.NewStore()
	if err != nil {
		t.Fatalf("failed to create new store: %v", err)
	}
}

func TestRegisterSchema(t *testing.T) {
	_, clean := server(t)
	defer clean()
	client := client(t)

	storeID, _ := client.NewStore()
	err := client.RegisterSchema(storeID, "Person", schema)
	if err != nil {
		t.Fatalf("failed to register schema: %v", err)
	}
}

func TestStart(t *testing.T) {
	_, clean := server(t)
	defer clean()
	client := client(t)

	storeID, _ := client.NewStore()
	err := client.RegisterSchema(storeID, "Person", schema)
	checkErr(t, err)
	err = client.Start(storeID)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}
}

func TestStartFromAddress(t *testing.T) {
	_, clean := server(t)
	defer clean()
	client := client(t)

	storeID, _ := client.NewStore()
	err := client.RegisterSchema(storeID, "Person", schema)
	checkErr(t, err)

	// TODO: figure out how to test this
	// client.StartFromAddress()
}

func TestModelCreate(t *testing.T) {
	_, clean := server(t)
	defer clean()
	client := client(t)

	storeID, _ := client.NewStore()
	err := client.RegisterSchema(storeID, "Person", schema)
	checkErr(t, err)

	err = client.ModelCreate(storeID, "Person", adam)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}
}

func TestModelSave(t *testing.T) {
	_, clean := server(t)
	defer clean()
	client := client(t)

	storeID, _ := client.NewStore()
	err := client.RegisterSchema(storeID, "Person", schema)
	checkErr(t, err)

	err = client.ModelCreate(storeID, "Person", adam)
	checkErr(t, err)

	adam.age = 30
	err = client.ModelSave(storeID, "Person", adam)
	if err != nil {
		t.Fatalf("failed to save model: %v", err)
	}
}

func TestModelDelete(t *testing.T) {
	_, clean := server(t)
	defer clean()
	client := client(t)

	storeID, _ := client.NewStore()
	err := client.RegisterSchema(storeID, "Person", schema)
	checkErr(t, err)

	err = client.ModelCreate(storeID, "Person", adam)
	checkErr(t, err)

	err = client.ModelDelete(storeID, "Person", adam.ID)
	if err != nil {
		t.Fatalf("failed to delete model: %v", err)
	}
}

func TestModelHas(t *testing.T) {
	_, clean := server(t)
	defer clean()
	client := client(t)

	storeID, _ := client.NewStore()
	err := client.RegisterSchema(storeID, "Person", schema)
	checkErr(t, err)

	err = client.ModelCreate(storeID, "Person", adam)
	checkErr(t, err)

	exists, err := client.ModelHas(storeID, "Person", adam.ID)
	if err != nil {
		t.Fatalf("failed to check model has: %v", err)
	}
	if !exists {
		t.Fatal("model should exist but it doesn't")
	}
}

func TestModelFind(t *testing.T) {

}

func TestModelFindByID(t *testing.T) {
	_, clean := server(t)
	defer clean()
	client := client(t)

	storeID, _ := client.NewStore()
	err := client.RegisterSchema(storeID, "Person", schema)
	checkErr(t, err)

	err = client.ModelCreate(storeID, "Person", adam)
	checkErr(t, err)

	newPerson := &Person{}
	err = client.ModelFindByID(storeID, "Person", adam.ID, newPerson)
	if err != nil {
		t.Fatalf("failed to find model by id: %v", err)
	}
	// TODO: seems that the newPerson has the correct ID but default values for everything else
	if !reflect.DeepEqual(newPerson, adam) {
		t.Fatal("model found by id does't equal the original")
	}
}

func TestReadTransaction(t *testing.T) {
	_, clean := server(t)
	defer clean()
	client := client(t)

	storeID, err := client.NewStore()
	checkErr(t, err)
	err = client.RegisterSchema(storeID, "Person", schema)
	checkErr(t, err)
	err = client.ModelCreate(storeID, "Person", adam)
	checkErr(t, err)

	txn, err := client.ReadTransaction(storeID, "Person")
	if err != nil {
		t.Fatalf("failed to create read txn: %v", err)
	}

	err = txn.Start()
	if err != nil {
		t.Fatalf("failed to start read txn: %v", err)
	}

	has, err := txn.Has(adam.ID)
	if err != nil {
		t.Fatalf("failed to read txn has: %v", err)
	}
	if !has {
		t.Fatal("expected has to be true but it wasn't")
	}

	newPerson := &Person{}
	err = txn.FindByID(adam.ID, newPerson)
	if err != nil {
		t.Fatalf("failed to txn find by id: %v", err)
	}
	// TODO: seems that the newPerson has the correct ID but default values for everything else
	if !reflect.DeepEqual(newPerson, adam) {
		t.Fatal("txn model found by id does't equal the original")
	}

	err = txn.End()
	if err != nil {
		t.Fatalf("failed to end txn: %v", err)
	}
}

func TestListen(t *testing.T) {
	_, clean := server(t)
	defer clean()
	client := client(t)

	storeID, _ := client.NewStore()
	err := client.RegisterSchema(storeID, "Person", schema)
	checkErr(t, err)

	err = client.ModelCreate(storeID, "Person", adam)
	checkErr(t, err)

	channel, err := client.Listen(storeID, "Person", adam.ID, &Person{})
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	go func() {
		adam.age = 30
		_ = client.ModelSave(storeID, "Person", adam)
		adam.age = 40
		_ = client.ModelSave(storeID, "Person", adam)
	}()

	val, ok := <-channel
	if !ok {
		t.Fatal("channel no longer active at first event")
	} else {
		fmt.Println(val, ok)
	}

	val, ok = <-channel
	if !ok {
		t.Fatal("channel no longer active at second event")
	} else {
		fmt.Println(val, ok)
	}
}

func server(t *testing.T) (*api.Server, func()) {
	dir := "/tmp/threads"
	ts, err := es.DefaultThreadservice(
		dir,
		es.ListenPort(4006),
		es.ProxyPort(5050),
		es.Debug(true))
	checkErr(t, err)
	ts.Bootstrap(util.DefaultBoostrapPeers())

	server, err := api.NewServer(context.Background(), ts, api.Config{
		RepoPath: dir,
		Debug:    true,
	})
	checkErr(t, err)
	return server, func() {
		if err := ts.Close(); err != nil {
			panic(err)
		}
		server.Close()
		os.RemoveAll(dir)
	}
}

func client(t *testing.T) *Client {
	client, err := NewClient("localhost", 9090)
	checkErr(t, err)
	return client
}

func checkErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
