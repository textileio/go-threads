package db

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/textileio/go-threads/common"
	lstore "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/util"
)

var (
	jsonSchema = `{
		"$schema": "http://json-schema.org/draft-04/schema#",
		"$ref": "#/definitions/person",
		"definitions": {
			"person": {
				"required": [
					"_id",
					"name",
					"age"
				],
				"properties": {
					"_id": {
						"type": "string"
					},
					"name": {
						"type": "string"
					},
					"age": {
						"type": "integer"
					}
				},
				"additionalProperties": false,
				"type": "object"
			}
		}
	}`
)

func TestManager_GetToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	man, clean := createTestManager(t)
	defer clean()

	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	checkErr(t, err)
	tok, err := man.GetToken(ctx, thread.NewLibp2pIdentity(sk))
	checkErr(t, err)
	if tok == "" {
		t.Fatal("bad token")
	}
}

func TestManager_NewDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t.Run("test one new db", func(t *testing.T) {
		t.Parallel()
		man, clean := createTestManager(t)
		defer clean()
		_, err := man.NewDB(ctx, thread.NewIDV1(thread.Raw, 32))
		checkErr(t, err)
	})
	t.Run("test multiple new dbs", func(t *testing.T) {
		t.Parallel()
		man, clean := createTestManager(t)
		defer clean()
		_, err := man.NewDB(ctx, thread.NewIDV1(thread.Raw, 32))
		checkErr(t, err)
		// NewDB with token
		sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
		checkErr(t, err)
		tok, err := man.GetToken(ctx, thread.NewLibp2pIdentity(sk))
		checkErr(t, err)
		_, err = man.NewDB(ctx, thread.NewIDV1(thread.Raw, 32), WithNewManagedToken(tok))
		checkErr(t, err)
	})
	t.Run("test new db with bad name", func(t *testing.T) {
		t.Parallel()
		man, clean := createTestManager(t)
		defer clean()
		name := "my db"
		_, err := man.NewDB(ctx, thread.NewIDV1(thread.Raw, 32), WithNewManagedName(name))
		if err == nil {
			t.Fatal("new db with bad name should fail")
		}
	})
	t.Run("test new db with name", func(t *testing.T) {
		t.Parallel()
		man, clean := createTestManager(t)
		defer clean()
		name := "my-db"
		d, err := man.NewDB(ctx, thread.NewIDV1(thread.Raw, 32), WithNewManagedName(name))
		checkErr(t, err)
		if d.name != name {
			t.Fatalf("expected name %s, got %s", name, d.name)
		}
	})
}

func TestManager_ReloadDBs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	dir := t.TempDir()
	n, err := common.DefaultNetwork(
		common.WithNetBadgerPersistence(dir),
		common.WithNetHostAddr(util.FreeLocalAddr()),
		common.WithNetDebug(true),
	)
	checkErr(t, err)
	store, err := util.NewBadgerDatastore(dir, "eventstore", false)
	checkErr(t, err)
	man, err := NewManager(store, n, WithNewDebug(true))
	checkErr(t, err)
	defer func() {
		store.Close()
	}()

	for i := 0; i < 2000; i++ {
		_ = createDBAndAddData(t, ctx, man)
	}
	time.Sleep(time.Second)

	// Close manager
	err = man.Close()
	checkErr(t, err)
	err = n.Close()
	checkErr(t, err)

	// Restart mananger
	n, err = common.DefaultNetwork(
		common.WithNetBadgerPersistence(dir),
		common.WithNetHostAddr(util.FreeLocalAddr()),
		common.WithNetDebug(true),
	)
	checkErr(t, err)
	man, err = NewManager(store, n, WithNewDebug(true))
	checkErr(t, err)
}

func TestManager_GetDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	dir := t.TempDir()
	n, err := common.DefaultNetwork(
		common.WithNetBadgerPersistence(dir),
		common.WithNetHostAddr(util.FreeLocalAddr()),
		common.WithNetDebug(true),
	)
	checkErr(t, err)
	store, err := util.NewBadgerDatastore(dir, "eventstore", false)
	checkErr(t, err)
	man, err := NewManager(store, n, WithNewDebug(true))
	checkErr(t, err)
	defer func() {
		store.Close()
	}()

	id := thread.NewIDV1(thread.Raw, 32)
	_, err = man.GetDB(ctx, id)
	if !errors.Is(err, lstore.ErrThreadNotFound) {
		t.Fatal("should be not found error")
	}

	_, err = man.NewDB(ctx, id)
	checkErr(t, err)
	db, err := man.GetDB(ctx, id)
	checkErr(t, err)
	if db == nil {
		t.Fatal("db not found")
	}

	// Register a schema and create an instance
	collection, err := db.NewCollection(CollectionConfig{Name: "Person", Schema: util.SchemaFromSchemaString(jsonSchema)})
	checkErr(t, err)
	person1 := []byte(`{"_id": "", "name": "foo", "age": 21}`)
	_, err = collection.Create(person1)
	checkErr(t, err)

	time.Sleep(time.Second)

	// Close it down, restart next
	err = man.Close()
	checkErr(t, err)
	err = n.Close()
	checkErr(t, err)

	t.Run("test get db after restart", func(t *testing.T) {
		n, err := common.DefaultNetwork(
			common.WithNetBadgerPersistence(dir),
			common.WithNetHostAddr(util.FreeLocalAddr()),
			common.WithNetDebug(true),
		)
		checkErr(t, err)
		man, err := NewManager(store, n, WithNewDebug(true))
		checkErr(t, err)

		db, err := man.GetDB(ctx, id)
		checkErr(t, err)
		if db == nil {
			t.Fatal("db was not hydrated")
		}

		// Add another instance, this time there should be no need to register the schema
		collection := db.GetCollection("Person")
		if collection == nil {
			t.Fatal("collection was not hydrated")
		}
		person2 := []byte(`{"_id": "", "name": "bar", "age": 21}`)
		person3 := []byte(`{"_id": "", "name": "baz", "age": 21}`)
		_, err = collection.CreateMany([][]byte{person2, person3})
		checkErr(t, err)

		// Delete the db, we'll try to restart again
		err = man.DeleteDB(ctx, id)
		checkErr(t, err)

		time.Sleep(time.Second)

		err = man.Close()
		checkErr(t, err)
		err = n.Close()
		checkErr(t, err)

		t.Run("test get deleted db after restart", func(t *testing.T) {
			n, err := common.DefaultNetwork(
				common.WithNetBadgerPersistence(dir),
				common.WithNetHostAddr(util.FreeLocalAddr()),
				common.WithNetDebug(true),
			)
			checkErr(t, err)
			man, err := NewManager(store, n, WithNewDebug(true))
			checkErr(t, err)

			_, err = man.GetDB(ctx, id)
			if !errors.Is(err, lstore.ErrThreadNotFound) {
				t.Fatal("db was not deleted")
			}

			time.Sleep(time.Second)

			err = man.Close()
			checkErr(t, err)
			err = n.Close()
			checkErr(t, err)
		})
	})
}

func TestManager_DeleteDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	man, clean := createTestManager(t)
	defer clean()

	id := thread.NewIDV1(thread.Raw, 32)
	db, err := man.NewDB(ctx, id)
	checkErr(t, err)

	// Register a schema and create an instance
	collection, err := db.NewCollection(CollectionConfig{Name: "Person", Schema: util.SchemaFromSchemaString(jsonSchema)})
	checkErr(t, err)
	person1 := []byte(`{"_id": "", "name": "foo", "age": 21}`)
	_, err = collection.Create(person1)
	checkErr(t, err)

	time.Sleep(time.Second)

	err = man.DeleteDB(ctx, id)
	checkErr(t, err)

	_, err = man.GetDB(ctx, id)
	if !errors.Is(err, lstore.ErrThreadNotFound) {
		t.Fatal("db was not deleted")
	}
}

func createTestManager(t *testing.T) (*Manager, func()) {
	dir := t.TempDir()
	n, err := common.DefaultNetwork(
		common.WithNetBadgerPersistence(dir),
		common.WithNetHostAddr(util.FreeLocalAddr()),
		common.WithNetDebug(true),
	)
	checkErr(t, err)
	store, err := util.NewBadgerDatastore(dir, "eventstore", false)
	checkErr(t, err)
	m, err := NewManager(store, n, WithNewDebug(true))
	checkErr(t, err)
	return m, func() {
		if err := n.Close(); err != nil {
			panic(err)
		}
		if err := m.Close(); err != nil {
			panic(err)
		}
		if err := store.Close(); err != nil {
			panic(err)
		}
	}
}

func createDBAndAddData(t *testing.T, ctx context.Context, man *Manager) *DB {
	db, err := man.NewDB(ctx, thread.NewIDV1(thread.Raw, 32))
	checkErr(t, err)

	collection, err := db.NewCollection(CollectionConfig{Name: "Person", Schema: util.SchemaFromSchemaString(jsonSchema)})
	checkErr(t, err)

	person1 := []byte(`{"_id": "", "name": "foo", "age": 21}`)
	_, err = collection.Create(person1)
	checkErr(t, err)

	return db
}
