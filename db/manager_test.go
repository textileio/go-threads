package db

import (
	"io/ioutil"
	"os"
	"testing"
	"time"
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
	t.Run("Single", func(t *testing.T) {
		t.Parallel()
		man, clean := createTestManager(t)
		defer clean()
		_, _, err := man.NewDB()
		checkErr(t, err)
	})
	t.Run("Multiple", func(t *testing.T) {
		t.Parallel()
		man, clean := createTestManager(t)
		defer clean()
		_, _, err := man.NewDB()
		checkErr(t, err)
		_, _, err = man.NewDB()
		checkErr(t, err)
	})
}

func TestManager_GetDB(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "")
	checkErr(t, err)
	ts, err := DefaultService(dir)
	checkErr(t, err)
	man, err := NewManager(ts, WithRepoPath(dir), WithJsonMode(true), WithDebug(true))
	checkErr(t, err)
	defer func() {
		_ = os.RemoveAll(dir)
	}()

	id, _, err := man.NewDB()
	checkErr(t, err)
	db := man.GetDB(id)
	if db == nil {
		t.Fatal("db not found")
	}

	// Register a schema, start, and create an instance
	model, err := db.RegisterSchema("Person", jsonSchema)
	checkErr(t, err)
	err = db.Start()
	checkErr(t, err)
	person1 := `{"ID": "", "Name": "Foo", "Age": 21}`
	err = model.Create(&person1)
	checkErr(t, err)

	time.Sleep(time.Second)

	// Close it down, restart next
	err = man.Close()
	checkErr(t, err)
	err = ts.Close()
	checkErr(t, err)

	t.Run("GetHydrated", func(t *testing.T) {
		ts, err := DefaultService(dir)
		checkErr(t, err)
		man, err := NewManager(ts, WithRepoPath(dir), WithJsonMode(true), WithDebug(true))
		checkErr(t, err)

		db := man.GetDB(id)
		if db == nil {
			t.Fatal("db was not hydrated")
		}

		// Add another instance, this time there should be no need to register the schema
		model := db.GetModel("Person")
		if model == nil {
			t.Fatal("model was not hydrated")
		}
		person2 := `{"ID": "", "Name": "Bar", "Age": 21}`
		person3 := `{"ID": "", "Name": "Baz", "Age": 21}`
		err = model.Create(&person2, &person3)
		checkErr(t, err)

		time.Sleep(time.Second)

		err = man.Close()
		checkErr(t, err)
		err = ts.Close()
		checkErr(t, err)
	})
}

func createTestManager(t *testing.T) (*Manager, func()) {
	dir, err := ioutil.TempDir("", "")
	checkErr(t, err)
	ts, err := DefaultService(dir)
	checkErr(t, err)
	m, err := NewManager(ts, WithRepoPath(dir), WithJsonMode(true), WithDebug(true))
	checkErr(t, err)
	return m, func() {
		if err := ts.Close(); err != nil {
			panic(err)
		}
		if err := m.Close(); err != nil {
			panic(err)
		}
		_ = os.RemoveAll(dir)
	}
}
