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
			Schema: util.SchemaFromInstance(&Dog{}),
		})
		checkErr(t, err)
	})
	t.Run("Multiple", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		_, err := db.NewCollection(CollectionConfig{
			Name:   "Dog",
			Schema: util.SchemaFromInstance(&Dog{}),
		})
		checkErr(t, err)
		_, err = db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}),
		})
		checkErr(t, err)
	})
	t.Run("Fail/WithoutInstanceID", func(t *testing.T) {
		t.Parallel()
		type FailingType struct {
			IDontHaveAnIDField int
		}
		db, clean := createTestDB(t)
		defer clean()
		cc := CollectionConfig{
			Name:   "FailingType",
			Schema: util.SchemaFromInstance(&FailingType{}),
		}
		if _, err := db.NewCollection(cc); err != ErrInvalidCollectionType {
			t.Fatal("the collection should be invalid")
		}
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
			Schema: util.SchemaFromInstance(&Person{}),
		})
		checkErr(t, err)

		t.Run("WithImplicitTx", func(t *testing.T) {
			newPerson := &Person{Name: "Foo", Age: 42}
			err = collection.Create(util.JSONStringFromInstance(newPerson))
			checkErr(t, err)
			assertPersonInCollection(t, collection, newPerson)
		})
		t.Run("WithTx", func(t *testing.T) {
			newPerson := &Person{Name: "Foo", Age: 42}
			err = collection.WriteTxn(func(txn *Txn) error {
				return txn.Create(util.JSONStringFromInstance(newPerson))
			})
			checkErr(t, err)
			assertPersonInCollection(t, collection, newPerson)
		})
	})
	t.Run("Multiple", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		collection, err := db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}),
		})
		checkErr(t, err)

		newPerson1 := &Person{Name: "Foo1", Age: 42}
		newPerson2 := &Person{Name: "Foo2", Age: 43}
		err = collection.WriteTxn(func(txn *Txn) error {
			err := txn.Create(util.JSONStringFromInstance(newPerson1))
			if err != nil {
				return err
			}
			return txn.Create(util.JSONStringFromInstance(newPerson2))
		})
		checkErr(t, err)
		assertPersonInCollection(t, collection, newPerson1)
		assertPersonInCollection(t, collection, newPerson2)
	})

	t.Run("WithDefinedID", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		collection, err := db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}),
		})
		checkErr(t, err)

		definedID := core.NewInstanceID()
		newPerson := &Person{ID: definedID, Name: "Foo1", Age: 42}
		checkErr(t, collection.Create(util.JSONStringFromInstance(newPerson)))

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
			Schema: util.SchemaFromInstance(&Person{}),
		})
		checkErr(t, err)

		p := &Person{Name: "Foo1", Age: 42}
		checkErr(t, m.Create(util.JSONStringFromInstance(p)))
		p2 := &Person{ID: p.ID, Name: "Fool2", Age: 43}
		if err = m.Create(util.JSONStringFromInstance(p2)); !errors.Is(err, errCantCreateExistingInstance) {
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
			Schema: util.SchemaFromInstance(&Person{}),
		})
		checkErr(t, err)
		p := &Person{Name: "Foo1", Age: 42}
		err = m.ReadTxn(func(txn *Txn) error {
			return txn.Create(util.JSONStringFromInstance(p))
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
			Schema: util.SchemaFromInstance(&Person{}),
		})
		checkErr(t, err)
		p := &Person{Name: "Foo1", Age: 42}
		checkErr(t, m.Create(util.JSONStringFromInstance(p)))
		err = m.ReadTxn(func(txn *Txn) error {
			return txn.Save(util.JSONStringFromInstance(p))
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
			Schema: util.SchemaFromInstance(&Person{}),
		})
		checkErr(t, err)
		p := &Person{Name: "Foo1", Age: 42}
		checkErr(t, m.Create(util.JSONStringFromInstance(p)))
		err = m.ReadTxn(func(txn *Txn) error {
			return txn.Delete(p.ID)
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
		Schema: util.SchemaFromInstance(&Person{}),
	})
	checkErr(t, err)

	p1 := &Person{Name: "Foo1", Age: 42}
	p2 := &Person{Name: "Foo2", Age: 43}
	p3 := &Person{Name: "Foo3", Age: 44}
	checkErr(t, m.Create(util.JSONStringFromInstance(p1), util.JSONStringFromInstance(p2), util.JSONStringFromInstance(p3)))
	assertPersonInCollection(t, m, p1)
	assertPersonInCollection(t, m, p2)
	assertPersonInCollection(t, m, p3)

	p1.Age, p2.Age, p3.Age = 51, 52, 53
	checkErr(t, m.Save(util.JSONStringFromInstance(p1), util.JSONStringFromInstance(p2), util.JSONStringFromInstance(p3)))
	assertPersonInCollection(t, m, p1)
	assertPersonInCollection(t, m, p2)
	assertPersonInCollection(t, m, p3)

	checkErr(t, m.Delete(p1.ID, p2.ID, p3.ID))
	exist1, err := m.Has(p1.ID)
	checkErr(t, err)
	exist2, err := m.Has(p1.ID)
	checkErr(t, err)
	exist3, err := m.Has(p1.ID)
	checkErr(t, err)
	if exist1 || exist2 || exist3 {
		t.Fatal("deleted instances shouldn't exist")
	}
}

func TestGetInstance(t *testing.T) {
	t.Parallel()

	db, clean := createTestDB(t)
	defer clean()
	collection, err := db.NewCollection(CollectionConfig{
		Name:   "Person",
		Schema: util.SchemaFromInstance(&Person{}),
	})
	checkErr(t, err)

	newPerson := &Person{Name: "Foo", Age: 42}
	newPersonString := util.JSONStringFromInstance(newPerson)
	err = collection.WriteTxn(func(txn *Txn) error {
		return txn.Create(newPersonString)
	})
	checkErr(t, err)

	t.Run("WithImplicitTx", func(t *testing.T) {
		var personString *string
		err = collection.FindByID(newPerson.ID, personString)
		checkErr(t, err)
		if newPersonString != personString {
			t.Fatalf(errInvalidInstanceState)
		}
	})
	t.Run("WithReadTx", func(t *testing.T) {
		var personString *string
		err = collection.ReadTxn(func(txn *Txn) error {
			err := txn.FindByID(newPerson.ID, personString)
			checkErr(t, err)
			if newPersonString != personString {
				t.Fatalf(errInvalidInstanceState)
			}
			return nil
		})
		checkErr(t, err)
	})
	t.Run("WithUpdateTx", func(t *testing.T) {
		var personString *string
		err = collection.WriteTxn(func(txn *Txn) error {
			err := txn.FindByID(newPerson.ID, personString)
			checkErr(t, err)
			if newPersonString != personString {
				t.Fatalf(errInvalidInstanceState)
			}
			return nil
		})
		checkErr(t, err)
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
			Schema: util.SchemaFromInstance(&Person{}),
		})
		checkErr(t, err)

		newPerson := &Person{Name: "Alice", Age: 42}
		newPersonString := util.JSONStringFromInstance(newPerson)
		err = collection.WriteTxn(func(txn *Txn) error {
			return txn.Create(newPersonString)
		})
		checkErr(t, err)

		err = collection.WriteTxn(func(txn *Txn) error {
			var personString *string
			err := txn.FindByID(newPerson.ID, personString)
			checkErr(t, err)

			p := &Person{}
			util.InstanceFromJSONString(personString, p)

			p.Name = "Bob"
			return txn.Save(util.JSONStringFromInstance(p))
		})
		checkErr(t, err)

		var personString *string
		err = collection.FindByID(newPerson.ID, personString)
		checkErr(t, err)
		person := &Person{}
		util.InstanceFromJSONString(personString, person)
		if person.ID != newPerson.ID || person.Age != 42 || person.Name != "Bob" {
			t.Fatalf(errInvalidInstanceState)
		}
	})
	t.Run("SaveNonExistant", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		m, err := db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}),
		})
		checkErr(t, err)

		p := &Person{Name: "Alice", Age: 42}
		if err := m.Save(util.JSONStringFromInstance(p)); !errors.Is(err, errCantSaveNonExistentInstance) {
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
		Schema: util.SchemaFromInstance(&Person{}),
	})
	checkErr(t, err)

	newPerson := &Person{Name: "Alice", Age: 42}
	newPersonString := util.JSONStringFromInstance(newPerson)
	err = collection.WriteTxn(func(txn *Txn) error {
		return txn.Create(newPersonString)
	})
	checkErr(t, err)

	createdPerson := &Person{}
	util.InstanceFromJSONString(newPersonString, createdPerson)

	err = collection.Delete(createdPerson.ID)
	checkErr(t, err)

	var personString *string
	if err = collection.FindByID(createdPerson.ID, personString); err != ErrNotFound {
		t.Fatalf("FindByID: instance shouldn't exist")
	}
	if exist, err := collection.Has(createdPerson.ID); exist || err != nil {
		t.Fatalf("Has: instance shouldn't exist")
	}

	// Try to delete again
	if err = collection.Delete(createdPerson.ID); err != ErrNotFound {
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
		Schema: util.SchemaFromInstance(&Person{}),
	})
	checkErr(t, err)
	t.Run("Create", func(t *testing.T) {
		f := &PersonFake{Name: "fake"}
		if err := collection.Create(util.JSONStringFromInstance(f)); !errors.Is(err, ErrInvalidSchemaInstance) {
			t.Fatalf("instance should be invalid compared to schema, got: %v", err)
		}
	})
	t.Run("Save", func(t *testing.T) {
		r := &Person{Name: "real"}
		err := collection.Create(util.JSONStringFromInstance(r))
		checkErr(t, err)
		f := &PersonFake{Name: "fake"}
		if err := collection.Save(util.JSONStringFromInstance(f)); !errors.Is(err, ErrInvalidSchemaInstance) {
			t.Fatalf("instance should be invalid compared to schema, got: %v", err)
		}
	})
}

func assertPersonInCollection(t *testing.T, collection *Collection, person *Person) {
	t.Helper()
	var personString *string
	err := collection.FindByID(person.ID, personString)
	checkErr(t, err)
	p := &Person{}
	util.InstanceFromJSONString(personString, p)
	if !reflect.DeepEqual(person, p) {
		t.Fatalf(errInvalidInstanceState)
	}
}
