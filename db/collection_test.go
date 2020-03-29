package db

import (
	"errors"
	"os"
	"reflect"
	"testing"

	logging "github.com/ipfs/go-log"
	core "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/util"
)

const (
	errInvalidInstanceState = "invalid instance state"
)

type Person struct {
	ID   core.InstanceID
	Name string
	Age  int
}

type Dog struct {
	ID       core.InstanceID
	Name     string
	Comments []Comment
}
type Comment struct {
	Body string
}

func TestMain(m *testing.M) {
	_ = logging.SetLogLevel("*", "error")
	os.Exit(m.Run())
}

func TestNewCollection(t *testing.T) {
	t.Parallel()
	t.Run("Single", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		_, err := db.NewCollection(CollectionConfig{
			Name:   "Dog",
			Schema: util.SchemaFromInstance(&Dog{}, false),
		})
		checkErr(t, err)
	})
	t.Run("SingleExpandedSchemaStruct", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		_, err := db.NewCollection(CollectionConfig{
			Name:   "Dog",
			Schema: util.SchemaFromInstance(&Dog{}, true),
		})
		checkErr(t, err)
	})
	t.Run("Multiple", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		_, err := db.NewCollection(CollectionConfig{
			Name:   "Dog",
			Schema: util.SchemaFromInstance(&Dog{}, false),
		})
		checkErr(t, err)
		_, err = db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}, false),
		})
		checkErr(t, err)
	})
	type FailingType struct {
		IDontHaveAnIDField int
	}
	t.Run("Fail/WithoutInstanceID", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		cc := CollectionConfig{
			Name:   "FailingType",
			Schema: util.SchemaFromInstance(&FailingType{}, false),
		}
		if _, err := db.NewCollection(cc); err != ErrInvalidCollectionSchema {
			t.Fatal("the collection should be invalid")
		}
	})
	t.Run("Fail/WithoutInstanceIDExpandedSchemaStruct", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		cc := CollectionConfig{
			Name:   "FailingType",
			Schema: util.SchemaFromInstance(&FailingType{}, true),
		}
		if _, err := db.NewCollection(cc); err != ErrInvalidCollectionSchema {
			t.Fatal("the collection should be invalid")
		}
	})
}

func TestAddIndex(t *testing.T) {
	t.Parallel()
	t.Run("CreateDBAndCollection", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		collection, err := db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}, false),
		})
		checkErr(t, err)

		t.Run("AddNameUniqueIndex", func(t *testing.T) {
			err := collection.AddIndex(IndexConfig{Path: "Name", Unique: true})
			checkErr(t, err)
		})
		t.Run("AddAgeNonUniqueIndex", func(t *testing.T) {
			err := collection.AddIndex(IndexConfig{Path: "Age", Unique: false})
			checkErr(t, err)
		})
		t.Run("AddIDIndex", func(t *testing.T) {
			err := collection.AddIndex(IndexConfig{Path: "ID", Unique: true})
			checkErr(t, err)
		})
	})
}

func TestCreateInstance(t *testing.T) {
	t.Parallel()
	t.Run("Single", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		collection, err := db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}, false),
		})
		checkErr(t, err)

		t.Run("WithImplicitTx", func(t *testing.T) {
			newPerson := util.JSONFromInstance(Person{Name: "Foo", Age: 42})
			res, err := collection.Create(newPerson)
			checkErr(t, err)
			newPerson = util.SetJSONID(res[0], newPerson)
			assertPersonInCollection(t, collection, newPerson)
		})
		t.Run("WithTx", func(t *testing.T) {
			newPerson := util.JSONFromInstance(Person{Name: "Foo", Age: 42})
			var res []core.InstanceID
			err = collection.WriteTxn(func(txn *Txn) (err error) {
				res, err = txn.Create(newPerson)
				return
			})
			checkErr(t, err)
			newPerson = util.SetJSONID(res[0], newPerson)
			assertPersonInCollection(t, collection, newPerson)
		})
	})
	t.Run("Multiple", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		collection, err := db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}, false),
		})
		checkErr(t, err)

		newPerson1 := util.JSONFromInstance(Person{Name: "Foo1", Age: 42})
		newPerson2 := util.JSONFromInstance(Person{Name: "Foo2", Age: 43})
		var res1 []core.InstanceID
		var res2 []core.InstanceID
		err = collection.WriteTxn(func(txn *Txn) (err error) {
			res1, err = txn.Create(newPerson1)
			if err != nil {
				return
			}
			res2, err = txn.Create(newPerson2)
			return
		})
		checkErr(t, err)
		newPerson1 = util.SetJSONID(res1[0], newPerson1)
		newPerson2 = util.SetJSONID(res2[0], newPerson2)
		assertPersonInCollection(t, collection, newPerson1)
		assertPersonInCollection(t, collection, newPerson2)
	})

	t.Run("WithDefinedID", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		collection, err := db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}, false),
		})
		checkErr(t, err)

		definedID := core.NewInstanceID()
		newPerson := util.JSONFromInstance(&Person{ID: definedID, Name: "Foo1", Age: 42})
		_, err = collection.Create(newPerson)
		checkErr(t, err)

		exists, err := collection.Has(definedID)
		checkErr(t, err)
		if !exists {
			t.Fatal("manually defined instance ID should exist")
		}
		assertPersonInCollection(t, collection, newPerson)
	})

	t.Run("Re-Create", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		m, err := db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}, false),
		})
		checkErr(t, err)

		p := util.JSONFromInstance(Person{Name: "Foo1", Age: 42})
		res, err := m.Create(p)
		checkErr(t, err)
		p2 := util.JSONFromInstance(Person{ID: res[0], Name: "Fool2", Age: 43})
		_, err = m.Create(p2)
		if !errors.Is(err, errCantCreateExistingInstance) {
			t.Fatal("shouldn't create already existing instance")
		}
	})
}

func TestReadTxnValidation(t *testing.T) {
	t.Parallel()

	t.Run("TryCreate", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		m, err := db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}, false),
		})
		checkErr(t, err)
		err = m.ReadTxn(func(txn *Txn) error {
			_, err := txn.Create(util.JSONFromInstance(Person{Name: "Foo1", Age: 42}))
			return err
		})
		if !errors.Is(err, ErrReadonlyTx) {
			t.Fatal("shouldn't write on read-only transaction")
		}
	})
	t.Run("TrySave", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		m, err := db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}, false),
		})
		checkErr(t, err)
		p := util.JSONFromInstance(Person{Name: "Foo1", Age: 42})
		res, err := m.Create(p)
		checkErr(t, err)
		p = util.SetJSONID(res[0], p)
		err = m.ReadTxn(func(txn *Txn) error {
			return txn.Save(p)
		})
		if !errors.Is(err, ErrReadonlyTx) {
			t.Fatal("shouldn't write on read-only transaction")
		}
	})
	t.Run("TryDelete", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		m, err := db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}, false),
		})
		checkErr(t, err)
		p := util.JSONFromInstance(Person{Name: "Foo1", Age: 42})
		res, err := m.Create(p)
		checkErr(t, err)
		err = m.ReadTxn(func(txn *Txn) error {
			return txn.Delete(res[0])
		})
		if !errors.Is(err, ErrReadonlyTx) {
			t.Fatal("shouldn't write on read-only transaction")
		}
	})
}

func TestVariadic(t *testing.T) {
	t.Parallel()

	db, clean := createTestDB(t)
	defer clean()
	m, err := db.NewCollection(CollectionConfig{
		Name:   "Person",
		Schema: util.SchemaFromInstance(&Person{}, false),
	})
	checkErr(t, err)

	p0 := util.JSONFromInstance(&Person{Name: "Foo1", Age: 42})
	p1 := util.JSONFromInstance(&Person{Name: "Foo2", Age: 43})
	p2 := util.JSONFromInstance(&Person{Name: "Foo3", Age: 44})
	res, err := m.CreateMany([][]byte{p0, p1, p2})
	checkErr(t, err)
	p0 = util.SetJSONID(res[0], p0)
	p1 = util.SetJSONID(res[1], p1)
	p2 = util.SetJSONID(res[2], p2)
	assertPersonInCollection(t, m, p0)
	assertPersonInCollection(t, m, p1)
	assertPersonInCollection(t, m, p2)

	pp0 := &Person{}
	util.InstanceFromJSON(p0, pp0)
	pp1 := &Person{}
	util.InstanceFromJSON(p1, pp1)
	pp2 := &Person{}
	util.InstanceFromJSON(p2, pp2)
	pp0.Age, pp1.Age, pp2.Age = 51, 52, 53
	checkErr(t, m.SaveMany([][]byte{util.JSONFromInstance(pp0), util.JSONFromInstance(pp1), util.JSONFromInstance(pp2)}))
	assertPersonInCollection(t, m, util.JSONFromInstance(pp0))
	assertPersonInCollection(t, m, util.JSONFromInstance(pp1))
	assertPersonInCollection(t, m, util.JSONFromInstance(pp2))

	checkErr(t, m.DeleteMany([]core.InstanceID{pp0.ID, pp1.ID, pp2.ID}))
	exist0, err := m.Has(pp0.ID)
	checkErr(t, err)
	exist1, err := m.Has(pp1.ID)
	checkErr(t, err)
	exist2, err := m.Has(pp2.ID)
	checkErr(t, err)
	if exist0 || exist1 || exist2 {
		t.Fatal("deleted instances shouldn't exist")
	}
}

func TestGetInstance(t *testing.T) {
	t.Parallel()

	db, clean := createTestDB(t)
	defer clean()
	collection, err := db.NewCollection(CollectionConfig{
		Name:   "Person",
		Schema: util.SchemaFromInstance(&Person{}, false),
	})
	checkErr(t, err)

	newPerson := util.JSONFromInstance(Person{Name: "Foo", Age: 42})
	var res []core.InstanceID
	err = collection.WriteTxn(func(txn *Txn) (err error) {
		res, err = txn.Create(newPerson)
		return
	})
	checkErr(t, err)
	newPerson = util.SetJSONID(res[0], newPerson)
	newPersonInstance := &Person{}
	util.InstanceFromJSON(newPerson, newPersonInstance)

	t.Run("WithImplicitTx", func(t *testing.T) {
		found, err := collection.FindByID(res[0])
		checkErr(t, err)

		foundInstance := &Person{}
		util.InstanceFromJSON(found, foundInstance)

		if !reflect.DeepEqual(newPersonInstance, foundInstance) {
			t.Fatalf(errInvalidInstanceState)
		}
	})
	t.Run("WithReadTx", func(t *testing.T) {
		var found []byte
		err = collection.ReadTxn(func(txn *Txn) (err error) {
			found, err = txn.FindByID(res[0])
			return
		})
		checkErr(t, err)

		foundInstance := &Person{}
		util.InstanceFromJSON(found, foundInstance)

		if !reflect.DeepEqual(newPersonInstance, foundInstance) {
			t.Fatalf(errInvalidInstanceState)
		}
	})
	t.Run("WithWriteTx", func(t *testing.T) {
		var found []byte
		err = collection.WriteTxn(func(txn *Txn) (err error) {
			found, err = txn.FindByID(res[0])
			return
		})
		checkErr(t, err)

		foundInstance := &Person{}
		util.InstanceFromJSON(found, foundInstance)

		if !reflect.DeepEqual(newPersonInstance, foundInstance) {
			t.Fatalf(errInvalidInstanceState)
		}
	})
}

func TestSaveInstance(t *testing.T) {
	t.Parallel()

	t.Run("Simple", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		collection, err := db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}, false),
		})
		checkErr(t, err)

		newPerson := util.JSONFromInstance(Person{Name: "Alice", Age: 42})
		var res []core.InstanceID
		err = collection.WriteTxn(func(txn *Txn) (err error) {
			res, err = txn.Create(newPerson)
			return
		})
		checkErr(t, err)

		err = collection.WriteTxn(func(txn *Txn) error {
			instance, err := txn.FindByID(res[0])
			checkErr(t, err)

			p := &Person{}
			util.InstanceFromJSON(instance, p)

			p.Name = "Bob"
			return txn.Save(util.JSONFromInstance(p))
		})
		checkErr(t, err)

		instance, err := collection.FindByID(res[0])
		checkErr(t, err)
		person := &Person{}
		util.InstanceFromJSON(instance, person)
		if person.ID != res[0] || person.Age != 42 || person.Name != "Bob" {
			t.Fatalf(errInvalidInstanceState)
		}
	})
	t.Run("SaveNonExistant", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		m, err := db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}, false),
		})
		checkErr(t, err)

		p := util.JSONFromInstance(Person{Name: "Alice", Age: 42})
		if err := m.Save(p); !errors.Is(err, errCantSaveNonExistentInstance) {
			t.Fatal("shouldn't save non-existent instasnce")
		}
	})
}

func TestDeleteInstance(t *testing.T) {
	t.Parallel()

	db, clean := createTestDB(t)
	defer clean()
	collection, err := db.NewCollection(CollectionConfig{
		Name:   "Person",
		Schema: util.SchemaFromInstance(&Person{}, false),
	})
	checkErr(t, err)

	newPerson := util.JSONFromInstance(Person{Name: "Alice", Age: 42})
	var res []core.InstanceID
	err = collection.WriteTxn(func(txn *Txn) (err error) {
		res, err = txn.Create(newPerson)
		return
	})
	checkErr(t, err)

	err = collection.Delete(res[0])
	checkErr(t, err)

	_, err = collection.FindByID(res[0])
	if err != ErrNotFound {
		t.Fatalf("FindByID: instance shouldn't exist")
	}
	if exist, err := collection.Has(res[0]); exist || err != nil {
		t.Fatalf("Has: instance shouldn't exist")
	}

	// Try to delete again
	if err = collection.Delete(res[0]); err != ErrNotFound {
		t.Fatalf("cant't delete non-existent instance")
	}
}

type PersonFake struct {
	ID   core.InstanceID
	Name string
}

func TestInvalidActions(t *testing.T) {
	t.Parallel()

	db, clean := createTestDB(t)
	defer clean()
	collection, err := db.NewCollection(CollectionConfig{
		Name:   "Person",
		Schema: util.SchemaFromInstance(&Person{}, false),
	})
	checkErr(t, err)
	t.Run("Create", func(t *testing.T) {
		f := util.JSONFromInstance(PersonFake{Name: "fake"})
		if _, err := collection.Create(f); !errors.Is(err, ErrInvalidSchemaInstance) {
			t.Fatalf("instance should be invalid compared to schema, got: %v", err)
		}
	})
	t.Run("Save", func(t *testing.T) {
		r := util.JSONFromInstance(Person{Name: "real"})
		_, err := collection.Create(r)
		checkErr(t, err)
		f := util.JSONFromInstance(PersonFake{Name: "fake"})
		if err := collection.Save(f); !errors.Is(err, ErrInvalidSchemaInstance) {
			t.Fatalf("instance should be invalid compared to schema, got: %v", err)
		}
	})
}

func assertPersonInCollection(t *testing.T, collection *Collection, personBytes []byte) {
	t.Helper()
	person := &Person{}
	util.InstanceFromJSON(personBytes, person)
	res, err := collection.FindByID(person.ID)
	checkErr(t, err)
	p := &Person{}
	util.InstanceFromJSON(res, p)
	if !reflect.DeepEqual(person, p) {
		t.Fatalf(errInvalidInstanceState)
	}
}
