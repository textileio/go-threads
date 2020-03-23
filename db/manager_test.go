package db

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/textileio/go-threads/core/thread"
)

var (
	jsonSchema = `{
		"$schema": "http://json-schema.org/draft-04/schema#",
		"$ref": "#/definitions/person",
		"definitions": {
			"person": {
				"required": [
					"ID",
					"Name",
					"Age"
				],
				"properties": {
					"ID": {
						"type": "string"
					},
					"Name": {
						"type": "string"
					},
					"Age": {
						"type": "integer"
					}
				},
				"additionalProperties": false,
				"type": "object"
			}
		}
	}`
)

func TestManager_NewDB(t *testing.T) {
	t.Parallel()
	t.Run("test one new db", func(t *testing.T) {
		t.Parallel()
		man, clean := createTestManager(t)
		defer clean()
		_, err := man.NewDB(context.Background(), thread.NewIDV1(thread.Raw, 32))
		checkErr(t, err)
	})
	t.Run("test multiple new dbs", func(t *testing.T) {
		t.Parallel()
		man, clean := createTestManager(t)
		defer clean()
		_, err := man.NewDB(context.Background(), thread.NewIDV1(thread.Raw, 32))
		checkErr(t, err)
		_, err = man.NewDB(context.Background(), thread.NewIDV1(thread.Raw, 32))
		checkErr(t, err)
	})
}

func TestManager_GetDB(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "")
	checkErr(t, err)
	n, err := DefaultNetwork(dir)
	checkErr(t, err)
	man, err := NewManager(n, WithRepoPath(dir), WithDebug(true))
	checkErr(t, err)
	defer func() {
		_ = os.RemoveAll(dir)
	}()

	id := thread.NewIDV1(thread.Raw, 32)
	_, err = man.NewDB(context.Background(), id)
	checkErr(t, err)
	db := man.GetDB(id)
	if db == nil {
		t.Fatal("db not found")
	}

	// Register a schema and create an instance
	collection, err := db.NewCollection(CollectionConfig{Name: "Person", Schema: jsonSchema})
	checkErr(t, err)
	person1 := `{"ID": "", "Name": "Foo", "Age": 21}`
	err = collection.Create(&person1)
	checkErr(t, err)

	time.Sleep(time.Second)

	// Close it down, restart next
	err = man.Close()
	checkErr(t, err)
	err = n.Close()
	checkErr(t, err)

	t.Run("test get db after restart", func(t *testing.T) {
		n, err := DefaultNetwork(dir)
		checkErr(t, err)
		man, err := NewManager(n, WithRepoPath(dir), WithDebug(true))
		checkErr(t, err)

		db := man.GetDB(id)
		if db == nil {
			t.Fatal("db was not hydrated")
		}

		// Add another instance, this time there should be no need to register the schema
		collection := db.GetCollection("Person")
		if collection == nil {
			t.Fatal("collection was not hydrated")
		}
		person2 := `{"ID": "", "Name": "Bar", "Age": 21}`
		person3 := `{"ID": "", "Name": "Baz", "Age": 21}`
		err = collection.Create(&person2, &person3)
		checkErr(t, err)

		time.Sleep(time.Second)

		err = man.Close()
		checkErr(t, err)
		err = n.Close()
		checkErr(t, err)
	})
}

func TestManager_DeleteDB(t *testing.T) {
	t.Parallel()
	man, clean := createTestManager(t)
	defer clean()

	id := thread.NewIDV1(thread.Raw, 32)
	db, err := man.NewDB(context.Background(), id)
	checkErr(t, err)

	// Register a schema and create an instance
	collection, err := db.NewCollection(CollectionConfig{Name: "Person", Schema: jsonSchema})
	checkErr(t, err)
	person1 := `{"ID": "", "Name": "Foo", "Age": 21}`
	err = collection.Create(&person1)
	checkErr(t, err)

	time.Sleep(time.Second)

	err = man.DeleteDB(context.Background(), id)
	checkErr(t, err)

	if man.GetDB(id) != nil {
		t.Fatal("db was not deleted")
	}
}

func createTestManager(t *testing.T) (*Manager, func()) {
	dir, err := ioutil.TempDir("", "")
	checkErr(t, err)
	n, err := DefaultNetwork(dir)
	checkErr(t, err)
	m, err := NewManager(n, WithRepoPath(dir), WithDebug(true))
	checkErr(t, err)
	return m, func() {
		if err := n.Close(); err != nil {
			panic(err)
		}
		if err := m.Close(); err != nil {
			panic(err)
		}
		_ = os.RemoveAll(dir)
	}
}
