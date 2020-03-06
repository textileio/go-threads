package db

import (
	"errors"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	logging "github.com/ipfs/go-log"
	core "github.com/textileio/go-threads/core/db"
)

const (
	errInvalidInstanceState = "invalid instance state"
)

type Person struct {
	ID   core.EntityID
	Name string
	Age  int
}

type Dog struct {
	ID       core.EntityID
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

func TestSchemaRegistration(t *testing.T) {
	t.Parallel()
	t.Run("Single", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		_, err := db.Register("Dog", &Dog{})
		checkErr(t, err)
	})
	t.Run("Multiple", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		_, err := db.Register("Dog", &Dog{})
		checkErr(t, err)
		_, err = db.Register("Person", &Person{})
		checkErr(t, err)
	})
	t.Run("Fail/WithoutEntityID", func(t *testing.T) {
		t.Parallel()
		type FailingModel struct {
			IDontHaveAnIDField int
		}
		db, clean := createTestDB(t)
		defer clean()
		if _, err := db.Register("FailingModel", &FailingModel{}); err != ErrInvalidModel {
			t.Fatal("the model should be invalid")
		}
	})
}

func TestCreateInstance(t *testing.T) {
	t.Parallel()
	t.Run("Single", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		model, err := db.Register("Person", &Person{})
		checkErr(t, err)

		t.Run("WithImplicitTx", func(t *testing.T) {
			newPerson := &Person{Name: "Foo", Age: 42}
			err = model.Create(newPerson)
			checkErr(t, err)
			assertPersonInModel(t, model, newPerson)
		})
		t.Run("WithTx", func(t *testing.T) {
			newPerson := &Person{Name: "Foo", Age: 42}
			err = model.WriteTxn(func(txn *Txn) error {
				return txn.Create(newPerson)
			})
			checkErr(t, err)
			assertPersonInModel(t, model, newPerson)
		})
	})
	t.Run("Multiple", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		model, err := db.Register("Person", &Person{})
		checkErr(t, err)

		newPerson1 := &Person{Name: "Foo1", Age: 42}
		newPerson2 := &Person{Name: "Foo2", Age: 43}
		err = model.WriteTxn(func(txn *Txn) error {
			err := txn.Create(newPerson1)
			if err != nil {
				return err
			}
			return txn.Create(newPerson2)
		})
		checkErr(t, err)
		assertPersonInModel(t, model, newPerson1)
		assertPersonInModel(t, model, newPerson2)
	})

	t.Run("WithDefinedID", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		model, err := db.Register("Person", &Person{})
		checkErr(t, err)

		definedID := core.NewEntityID()
		newPerson := &Person{ID: definedID, Name: "Foo1", Age: 42}
		checkErr(t, model.Create(newPerson))

		exists, err := model.Has(definedID)
		checkErr(t, err)
		if !exists {
			t.Fatal("manually defined entity ID should exist")
		}
		assertPersonInModel(t, model, newPerson)
	})

	t.Run("Re-Create", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		m, err := db.Register("Person", &Person{})
		checkErr(t, err)

		p := &Person{Name: "Foo1", Age: 42}
		checkErr(t, m.Create(p))
		p2 := &Person{ID: p.ID, Name: "Fool2", Age: 43}
		if err = m.Create(p2); !errors.Is(err, errCantCreateExistingInstance) {
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
		m, err := db.Register("Person", &Person{})
		checkErr(t, err)
		p := &Person{Name: "Foo1", Age: 42}
		err = m.ReadTxn(func(txn *Txn) error {
			return txn.Create(p)
		})
		if !errors.Is(err, ErrReadonlyTx) {
			t.Fatal("shouldn't write on read-only transaction")
		}
	})
	t.Run("TrySave", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		m, err := db.Register("Person", &Person{})
		checkErr(t, err)
		p := &Person{Name: "Foo1", Age: 42}
		checkErr(t, m.Create(p))
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
		m, err := db.Register("Person", &Person{})
		checkErr(t, err)
		p := &Person{Name: "Foo1", Age: 42}
		checkErr(t, m.Create(p))
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
	m, err := db.Register("Person", &Person{})
	checkErr(t, err)

	p1 := &Person{Name: "Foo1", Age: 42}
	p2 := &Person{Name: "Foo2", Age: 43}
	p3 := &Person{Name: "Foo3", Age: 44}
	checkErr(t, m.Create(p1, p2, p3))
	assertPersonInModel(t, m, p1)
	assertPersonInModel(t, m, p2)
	assertPersonInModel(t, m, p3)

	p1.Age, p2.Age, p3.Age = 51, 52, 53
	checkErr(t, m.Save(p1, p2, p3))
	assertPersonInModel(t, m, p1)
	assertPersonInModel(t, m, p2)
	assertPersonInModel(t, m, p3)

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
	model, err := db.Register("Person", &Person{})
	checkErr(t, err)

	newPerson := &Person{Name: "Foo", Age: 42}
	err = model.WriteTxn(func(txn *Txn) error {
		return txn.Create(newPerson)
	})
	checkErr(t, err)

	t.Run("WithImplicitTx", func(t *testing.T) {
		person := &Person{}
		err = model.FindByID(newPerson.ID, person)
		checkErr(t, err)
		if !reflect.DeepEqual(newPerson, person) {
			t.Fatalf(errInvalidInstanceState)
		}
	})
	t.Run("WithReadTx", func(t *testing.T) {
		person := &Person{}
		err = model.ReadTxn(func(txn *Txn) error {
			err := txn.FindByID(newPerson.ID, person)
			checkErr(t, err)
			if !reflect.DeepEqual(newPerson, person) {
				t.Fatalf(errInvalidInstanceState)
			}
			return nil
		})
		checkErr(t, err)
	})
	t.Run("WithUpdateTx", func(t *testing.T) {
		person := &Person{}
		err = model.WriteTxn(func(txn *Txn) error {
			err := txn.FindByID(newPerson.ID, person)
			checkErr(t, err)
			if !reflect.DeepEqual(newPerson, person) {
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
		model, err := db.Register("Person", &Person{})
		checkErr(t, err)

		newPerson := &Person{Name: "Alice", Age: 42}
		err = model.WriteTxn(func(txn *Txn) error {
			return txn.Create(newPerson)
		})
		checkErr(t, err)

		err = model.WriteTxn(func(txn *Txn) error {
			p := &Person{}
			err := txn.FindByID(newPerson.ID, p)
			checkErr(t, err)

			p.Name = "Bob"
			return txn.Save(p)
		})
		checkErr(t, err)

		person := &Person{}
		err = model.FindByID(newPerson.ID, person)
		checkErr(t, err)
		if person.ID != newPerson.ID || person.Age != 42 || person.Name != "Bob" {
			t.Fatalf(errInvalidInstanceState)
		}
	})
	t.Run("SaveNonExistant", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		m, err := db.Register("Person", &Person{})
		checkErr(t, err)

		p := &Person{Name: "Alice", Age: 42}
		if err := m.Save(p); !errors.Is(err, errCantSaveNonExistentInstance) {
			t.Fatal("shouldn't save non-existent instasnce")
		}
	})
}

func TestDeleteInstance(t *testing.T) {
	t.Parallel()

	db, clean := createTestDB(t)
	defer clean()
	model, err := db.Register("Person", &Person{})
	checkErr(t, err)

	newPerson := &Person{Name: "Alice", Age: 42}
	err = model.WriteTxn(func(txn *Txn) error {
		return txn.Create(newPerson)
	})
	checkErr(t, err)

	err = model.Delete(newPerson.ID)
	checkErr(t, err)

	if err = model.FindByID(newPerson.ID, &Person{}); err != ErrNotFound {
		t.Fatalf("FindByID: instance shouldn't exist")
	}
	if exist, err := model.Has(newPerson.ID); exist || err != nil {
		t.Fatalf("Has: instance shouldn't exist")
	}

	// Try to delete again
	if err = model.Delete(newPerson.ID); err != ErrNotFound {
		t.Fatalf("cant't delete non-existent instance")
	}
}

type PersonFake struct {
	ID   core.EntityID
	Name string
}

func TestInvalidActions(t *testing.T) {
	t.Parallel()

	db, clean := createTestDB(t)
	defer clean()
	model, err := db.Register("Person", &Person{})
	checkErr(t, err)
	t.Run("Create", func(t *testing.T) {
		f := &PersonFake{Name: "fake"}
		if err := model.Create(f); !errors.Is(err, ErrInvalidSchemaInstance) {
			t.Fatalf("instance should be invalid compared to schema, got: %v", err)
		}
	})
	t.Run("Save", func(t *testing.T) {
		r := &Person{Name: "real"}
		err := model.Create(r)
		checkErr(t, err)
		f := &PersonFake{Name: "fake"}
		if err := model.Save(f); !errors.Is(err, ErrInvalidSchemaInstance) {
			t.Fatalf("instance should be invalid compared to schema, got: %v", err)
		}
	})
}

func checkErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func assertPersonInModel(t *testing.T, model *Model, person *Person) {
	t.Helper()
	p := &Person{}
	err := model.FindByID(person.ID, p)
	checkErr(t, err)
	if !reflect.DeepEqual(person, p) {
		t.Fatalf(errInvalidInstanceState)
	}
}

func createTestDB(t *testing.T, opts ...Option) (*DB, func()) {
	dir, err := ioutil.TempDir("", "")
	checkErr(t, err)
	ts, err := DefaultService(dir)
	checkErr(t, err)
	opts = append(opts, WithRepoPath(dir))
	s, err := NewDB(ts, opts...)
	checkErr(t, err)
	return s, func() {
		if err := ts.Close(); err != nil {
			panic(err)
		}
		_ = os.RemoveAll(dir)
	}
}
