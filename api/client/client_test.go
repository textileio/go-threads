package client

import (
	"context"
	"os"
	"testing"

	"github.com/textileio/go-textile-threads/api"
	es "github.com/textileio/go-textile-threads/eventstore"
	"github.com/textileio/go-textile-threads/util"
)

var (
// ts es.ThreadserviceBoostrapper
// server *api.Server
// client  *Client
// storeID string
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

const adam = `{
	"ID": "",
	"firstName": "Adam",
	"lastName": "Doe",
	"age": 21
}`

const eve = `{
	"ID": "",
	"firstName": "Eve",
	"lastName": "Doe",
	"age": 21
}`

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
	_ = client.RegisterSchema(storeID, "Person", schema)

	_, err := client.ModelCreate(storeID, "Person", []string{adam})
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
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
