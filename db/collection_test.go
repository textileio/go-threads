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
	// t.Run("Fail/WithoutInstanceID", func(t *testing.T) {
	// 	t.Parallel()
	// 	type FailingType struct {
	// 		IDontHaveAnIDField int
	// 	}
	// 	db, clean := createTestDB(t)
	// 	defer clean()
	// 	cc := CollectionConfig{
	// 		Name:   "FailingType",
	// 		Schema: util.SchemaFromInstance(&FailingType{}),
	// 	}
	// 	if _, err := db.NewCollection(cc); err != ErrInvalidCollectionType {
	// 		t.Fatal("the collection should be invalid")
	// 	}
	// })
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
			newPerson := util.JSONStringFromInstance(&Person{Name: "Foo", Age: 42})
			err = collection.Create(newPerson)
			checkErr(t, err)
			updatedNewPerson := &Person{}
			util.InstanceFromJSONString(*newPerson, updatedNewPerson)
			assertPersonInCollection(t, collection, updatedNewPerson)
		})
		t.Run("WithTx", func(t *testing.T) {
			newPerson := util.JSONStringFromInstance(&Person{Name: "Foo", Age: 42})
			err = collection.WriteTxn(func(txn *Txn) error {
				return txn.Create(newPerson)
			})
			checkErr(t, err)
			updatedNewPerson := &Person{}
			util.InstanceFromJSONString(*newPerson, updatedNewPerson)
			assertPersonInCollection(t, collection, updatedNewPerson)
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

		newPerson1 := util.JSONStringFromInstance(&Person{Name: "Foo1", Age: 42})
		newPerson2 := util.JSONStringFromInstance(&Person{Name: "Foo2", Age: 43})
		err = collection.WriteTxn(func(txn *Txn) error {
			err := txn.Create(newPerson1)
			if err != nil {
				return err
			}
			return txn.Create(newPerson2)
		})
		updatedNewPerson1 := &Person{}
		util.InstanceFromJSONString(*newPerson1, updatedNewPerson1)
		updatedNewPerson2 := &Person{}
		util.InstanceFromJSONString(*newPerson2, updatedNewPerson2)
		checkErr(t, err)
		assertPersonInCollection(t, collection, updatedNewPerson1)
		assertPersonInCollection(t, collection, updatedNewPerson2)
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
		pString := util.JSONStringFromInstance(p)
		checkErr(t, m.Create(pString))
		updatedP := &Person{}
		util.InstanceFromJSONString(*pString, updatedP)
		p2 := &Person{ID: updatedP.ID, Name: "Fool2", Age: 43}
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

	p1 := util.JSONStringFromInstance(&Person{Name: "Foo1", Age: 42})
	p2 := util.JSONStringFromInstance(&Person{Name: "Foo2", Age: 43})
	p3 := util.JSONStringFromInstance(&Person{Name: "Foo3", Age: 44})
	checkErr(t, m.Create(p1, p2, p3))
	// need to re-create the go structs from strings since they were assigned ids in Create
	pp1 := &Person{}
	util.InstanceFromJSONString(*p1, pp1)
	pp2 := &Person{}
	util.InstanceFromJSONString(*p2, pp2)
	pp3 := &Person{}
	util.InstanceFromJSONString(*p3, pp3)
	assertPersonInCollection(t, m, pp1)
	assertPersonInCollection(t, m, pp2)
	assertPersonInCollection(t, m, pp3)

	pp1.Age, pp2.Age, pp3.Age = 51, 52, 53
	checkErr(t, m.Save(util.JSONStringFromInstance(pp1), util.JSONStringFromInstance(pp2), util.JSONStringFromInstance(pp3)))
	assertPersonInCollection(t, m, pp1)
	assertPersonInCollection(t, m, pp2)
	assertPersonInCollection(t, m, pp3)

	checkErr(t, m.Delete(pp1.ID, pp2.ID, pp3.ID))
	exist1, err := m.Has(pp1.ID)
	checkErr(t, err)
	exist2, err := m.Has(pp2.ID)
	checkErr(t, err)
	exist3, err := m.Has(pp3.ID)
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

	newPerson := util.JSONStringFromInstance(&Person{Name: "Foo", Age: 42})
	err = collection.WriteTxn(func(txn *Txn) error {
		return txn.Create(newPerson)
	})
	checkErr(t, err)
	updatedNewPerson := &Person{}
	util.InstanceFromJSONString(*newPerson, updatedNewPerson)

	t.Run("WithImplicitTx", func(t *testing.T) {
		personString := ""
		err = collection.FindByID(updatedNewPerson.ID, &personString)
		checkErr(t, err)
		if *newPerson != personString {
			t.Fatalf(errInvalidInstanceState)
		}
	})
	t.Run("WithReadTx", func(t *testing.T) {
		personString := ""
		err = collection.ReadTxn(func(txn *Txn) error {
			err := txn.FindByID(updatedNewPerson.ID, &personString)
			checkErr(t, err)
			if *newPerson != personString {
				t.Fatalf(errInvalidInstanceState)
			}
			return nil
		})
		checkErr(t, err)
	})
	t.Run("WithWriteTx", func(t *testing.T) {
		personString := ""
		err = collection.WriteTxn(func(txn *Txn) error {
			err := txn.FindByID(updatedNewPerson.ID, &personString)
			checkErr(t, err)
			if *newPerson != personString {
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

		newPerson := util.JSONStringFromInstance(&Person{Name: "Alice", Age: 42})
		err = collection.WriteTxn(func(txn *Txn) error {
			return txn.Create(newPerson)
		})
		checkErr(t, err)
		updatedNewPerson := &Person{}
		util.InstanceFromJSONString(*newPerson, updatedNewPerson)

		err = collection.WriteTxn(func(txn *Txn) error {
			personString := ""
			err := txn.FindByID(updatedNewPerson.ID, &personString)
			checkErr(t, err)

			p := &Person{}
			util.InstanceFromJSONString(personString, p)

			p.Name = "Bob"
			return txn.Save(util.JSONStringFromInstance(p))
		})
		checkErr(t, err)

		personString := ""
		err = collection.FindByID(updatedNewPerson.ID, &personString)
		checkErr(t, err)
		person := &Person{}
		util.InstanceFromJSONString(personString, person)
		if person.ID != updatedNewPerson.ID || person.Age != 42 || person.Name != "Bob" {
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
	util.InstanceFromJSONString(*newPersonString, createdPerson)

	err = collection.Delete(createdPerson.ID)
	checkErr(t, err)

	personString := ""
	if err = collection.FindByID(createdPerson.ID, &personString); err != ErrNotFound {
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
	personString := ""
	err := collection.FindByID(person.ID, &personString)
	checkErr(t, err)
	p := &Person{}
	util.InstanceFromJSONString(personString, p)
	if !reflect.DeepEqual(person, p) {
		t.Fatalf(errInvalidInstanceState)
	}
}
