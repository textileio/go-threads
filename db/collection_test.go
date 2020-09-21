package db

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	logging "github.com/ipfs/go-log"
	core "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/util"
	"github.com/xeipuuv/gojsonschema"
)

const (
	errInvalidInstanceState = "invalid instance state"
)

type Person struct {
	ID   core.InstanceID `json:"_id"`
	Name string
	Age  int
}

type Person2 struct {
	ID       core.InstanceID `json:"_id"`
	Name     string
	Age      int
	Toys     Toys
	Comments []Comment
}

type Dog struct {
	ID       core.InstanceID `json:"_id"`
	Name     string
	Comments []Comment
}

type Dog2 struct {
	ID       core.InstanceID `json:"_id"`
	FullName string
	Breed    string
	Toys     Toys
	Comments []Comment
}

type Toys struct {
	Favorite string
	Names    []string
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
	t.Run("WithIndexes", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		c, err := db.NewCollection(CollectionConfig{
			Name:    "Dog",
			Schema:  util.SchemaFromInstance(&Dog{}, false),
			Indexes: []Index{{Path: "Name", Unique: false}},
		})
		checkErr(t, err)
		indexes := c.GetIndexes()
		if len(indexes) != 1 {
			t.Fatalf("expected %d indexes, got %d", 1, len(indexes))
		}
	})
	t.Run("WithNestedIndexes", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		c, err := db.NewCollection(CollectionConfig{
			Name:    "Dog",
			Schema:  util.SchemaFromInstance(&Dog2{}, false),
			Indexes: []Index{{Path: "FullName", Unique: true}, {Path: "Toys.Favorite"}},
		})
		checkErr(t, err)
		indexes := c.GetIndexes()
		if len(indexes) != 2 {
			t.Fatalf("expected %d indexes, got %d", 2, len(indexes))
		}
	})
	t.Run("WithWriteValidator", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		c, err := db.NewCollection(CollectionConfig{
			Name:   "Dog",
			Schema: util.SchemaFromInstance(&Dog{}, false),
			WriteValidator: `
				var type = event.patch.type
				var patch = event.patch.json_patch
				switch (type) {
				  case "delete":
				    return false
				  default:
				    if (patch.Name !== "Fido" && patch.Name != "Clyde") {
				      return false
				    }
				    return true
				}
			`,
		})
		checkErr(t, err)
		dog := Dog{Name: "Fido", Comments: []Comment{}}
		id, err := c.Create(util.JSONFromInstance(dog))
		checkErr(t, err)
		dog.ID = id
		dog.Name = "Bob"
		err = c.Save(util.JSONFromInstance(dog))
		if err == nil {
			t.Fatal("save should have been invalid")
		}
		dog.Name = "Clyde"
		err = c.Save(util.JSONFromInstance(dog))
		checkErr(t, err)
		err = c.Delete(dog.ID)
		if err == nil {
			t.Fatal("delete should have been invalid")
		}
	})
	t.Run("WithReadFilter", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		c, err := db.NewCollection(CollectionConfig{
			Name:   "Dog",
			Schema: util.SchemaFromInstance(&Dog{}, false),
			ReadFilter: `
				instance.Name = "Clyde"
				return instance
			`,
		})
		checkErr(t, err)
		dog := Dog{Name: "Fido", Comments: []Comment{}}
		id, err := c.Create(util.JSONFromInstance(dog))
		checkErr(t, err)
		res, err := c.FindByID(id)
		checkErr(t, err)
		filtered := Dog{}
		util.InstanceFromJSON(res, &filtered)
		if filtered.Name != "Clyde" {
			t.Fatal("name should have been modified by read filter")
		}
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
	t.Run("Fail/InvalidName", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		cc := CollectionConfig{
			Name:   "Not a URL-safe name",
			Schema: util.SchemaFromInstance(&Dog{}, false),
		}
		if _, err := db.NewCollection(cc); err != ErrInvalidName {
			t.Fatal("the collection name should be invalid")
		}
	})
	t.Run("Fail/BadIndexPath", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		_, err := db.NewCollection(CollectionConfig{
			Name:    "Dog",
			Schema:  util.SchemaFromInstance(&Dog{}, false),
			Indexes: []Index{{Path: "Missing", Unique: false}},
		})
		if err == nil {
			t.Fatal("index path should not be valid")
		}
	})
}

func TestUpdateCollection(t *testing.T) {
	t.Parallel()
	t.Run("AddFields", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		c, err := db.NewCollection(CollectionConfig{
			Name:   "Dog",
			Schema: util.SchemaFromInstance(&Dog{}, false),
		})
		checkErr(t, err)
		_, err = c.Create([]byte(`{"Name": "Fido", "Comments": []}`))
		checkErr(t, err)

		c, err = db.UpdateCollection(CollectionConfig{
			Name:   "Dog",
			Schema: util.SchemaFromInstance(&Dog2{}, false),
		})
		checkErr(t, err)
		_, err = c.Create([]byte(`{"Name": "Fido", "Comments": []}`))
		if err == nil {
			t.Fatal("instance should not be valid")
		}
		_, err = c.Create([]byte(`{"FullName": "Lassie", "Breed": "Collie", "Toys": {"Favorite": "Ball", "Names": ["Ball", "Frisbee"]}, "Comments": []}`))
		checkErr(t, err)

		dogs, err := c.Find(&Query{})
		checkErr(t, err)
		if len(dogs) != 2 {
			t.Fatalf("expected %d indexes, got %d", 2, len(dogs))
		}
		dog1 := &Dog2{}
		err = json.Unmarshal(dogs[0], dog1)
		checkErr(t, err)
		dog2 := &Dog2{}
		err = json.Unmarshal(dogs[1], dog2)
		checkErr(t, err)
	})
	t.Run("AddFieldsAndIndexes", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		_, err := db.NewCollection(CollectionConfig{
			Name:    "Dog",
			Schema:  util.SchemaFromInstance(&Dog{}, false),
			Indexes: []Index{{Path: "Name", Unique: true}},
		})
		checkErr(t, err)
		c, err := db.UpdateCollection(CollectionConfig{
			Name:    "Dog",
			Schema:  util.SchemaFromInstance(&Dog2{}, false),
			Indexes: []Index{{Path: "FullName", Unique: true}, {Path: "Toys.Favorite"}},
		})
		checkErr(t, err)
		indexes := c.GetIndexes()
		if len(indexes) != 2 {
			t.Fatalf("expected %d indexes, got %d", 2, len(indexes))
		}
	})
	t.Run("RemoveFieldsAndIndexes", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		_, err := db.NewCollection(CollectionConfig{
			Name:    "Dog",
			Schema:  util.SchemaFromInstance(&Dog2{}, false),
			Indexes: []Index{{Path: "FullName", Unique: true}, {Path: "Toys.Favorite"}},
		})
		checkErr(t, err)
		c, err := db.UpdateCollection(CollectionConfig{
			Name:    "Dog",
			Schema:  util.SchemaFromInstance(&Dog{}, false),
			Indexes: []Index{{Path: "Name", Unique: true}},
		})
		checkErr(t, err)
		indexes := c.GetIndexes()
		if len(indexes) != 1 {
			t.Fatalf("expected %d indexes, got %d", 1, len(indexes))
		}
	})
	t.Run("Fail/BadIndexPath", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		_, err := db.NewCollection(CollectionConfig{
			Name:   "Dog",
			Schema: util.SchemaFromInstance(&Dog{}, false),
		})
		checkErr(t, err)
		_, err = db.UpdateCollection(CollectionConfig{
			Name:    "Dog",
			Schema:  util.SchemaFromInstance(&Dog{}, false),
			Indexes: []Index{{Path: "Missing", Unique: false}},
		})
		if err == nil {
			t.Fatal("index path should not be valid")
		}
	})
}

func TestDeleteCollection(t *testing.T) {
	t.Parallel()
	db, clean := createTestDB(t)
	defer clean()
	_, err := db.NewCollection(CollectionConfig{
		Name:    "Dog",
		Schema:  util.SchemaFromInstance(&Dog2{}, false),
		Indexes: []Index{{Path: "FullName", Unique: true}},
	})
	checkErr(t, err)
	err = db.DeleteCollection("Dog")
	checkErr(t, err)
	if db.GetCollection("Dog") != nil {
		t.Fatal("collection should be deleted")
	}
}

func TestAddIndex(t *testing.T) {
	t.Parallel()
	db, clean := createTestDB(t)
	defer clean()
	schema := util.SchemaFromInstance(&Person2{}, false)
	c, err := db.NewCollection(CollectionConfig{
		Name:   "Person",
		Schema: schema,
	})
	checkErr(t, err)

	t.Run("AddNameUniqueIndex", func(t *testing.T) {
		err := c.addIndex(schema, Index{Path: "Name", Unique: false})
		checkErr(t, err)
	})
	t.Run("AddAgeNonUniqueIndex", func(t *testing.T) {
		err := c.addIndex(schema, Index{Path: "Age", Unique: false})
		checkErr(t, err)
	})
	t.Run("AddNestedIndex", func(t *testing.T) {
		err := c.addIndex(schema, Index{Path: "Toys.Favorite", Unique: false})
		checkErr(t, err)
	})
	t.Run("Fail/AddIndexWithBadPath", func(t *testing.T) {
		err := c.addIndex(schema, Index{Path: "Bad.Path", Unique: false})
		if err == nil {
			t.Fatal("index path should not be valid")
		}
	})
	t.Run("Fail/AddIndexOnRef", func(t *testing.T) {
		err := c.addIndex(schema, Index{Path: "Toys", Unique: false})
		if err == nil {
			t.Fatal("index path should not be valid")
		}
	})
	t.Run("Fail/AddIndexOnBadType", func(t *testing.T) {
		err := c.addIndex(schema, Index{Path: "Comments", Unique: false})
		if err == nil {
			t.Fatal("index path should not be valid")
		}
	})
	t.Run("Fail/AddUniqueIndexOnNonUniquePath", func(t *testing.T) {
		_, err := c.Create(util.JSONFromInstance(Person2{Name: "Foo", Age: 42, Toys: Toys{Favorite: "", Names: []string{}}, Comments: []Comment{}}))
		checkErr(t, err)
		_, err = c.Create(util.JSONFromInstance(Person2{Name: "Foo", Age: 42, Toys: Toys{Favorite: "", Names: []string{}}, Comments: []Comment{}}))
		checkErr(t, err)
		err = c.addIndex(schema, Index{Path: "Name", Unique: true})
		if err == nil {
			t.Fatal("index path should not be valid")
		}
	})
}

func TestEmptySchema(t *testing.T) {
	t.Parallel()
	db, clean := createTestDB(t)
	defer clean()
	// Note the empty schema {}, this is a catch all schema
	schema := util.SchemaFromSchemaString("{}")
	_, err := db.NewCollection(CollectionConfig{
		Name:   "Person",
		Schema: schema,
	})
	if err != ErrInvalidCollectionSchema {
		t.Fatalf("expected to throw ErrInvalidCollectionSchema error")
	}
}

func TestGetName(t *testing.T) {
	t.Parallel()
	db, clean := createTestDB(t)
	defer clean()
	schema := util.SchemaFromInstance(&Person2{}, false)
	c, err := db.NewCollection(CollectionConfig{
		Name:   "Person",
		Schema: schema,
	})
	checkErr(t, err)
	name := c.GetName()
	if name != "Person" {
		t.Fatalf("expected Person, but got %s", name)
	}
}

func TestGetSchema(t *testing.T) {
	t.Parallel()
	db, clean := createTestDB(t)
	defer clean()
	schema := util.SchemaFromInstance(&Person2{}, false)
	c, err := db.NewCollection(CollectionConfig{
		Name:   "Person",
		Schema: schema,
	})
	checkErr(t, err)
	sb, err := json.Marshal(schema)
	checkErr(t, err)
	original := gojsonschema.NewBytesLoader(sb).JsonSource().([]byte)
	got := c.GetSchema()
	if !bytes.Equal(got, original) {
		t.Fatal("got schema does not match original")
	}
}

func TestGetWriteValidator(t *testing.T) {
	t.Parallel()
	db, clean := createTestDB(t)
	defer clean()
	schema := util.SchemaFromInstance(&Person2{}, false)
	wv := "return true"
	c, err := db.NewCollection(CollectionConfig{
		Name:           "Person",
		Schema:         schema,
		WriteValidator: wv,
	})
	checkErr(t, err)
	if !bytes.Equal(c.GetWriteValidator(), []byte(wv)) {
		t.Fatal("got write validator does not match original")
	}
}

func TestGetReadFilter(t *testing.T) {
	t.Parallel()
	db, clean := createTestDB(t)
	defer clean()
	schema := util.SchemaFromInstance(&Person2{}, false)
	rf := "return instance"
	c, err := db.NewCollection(CollectionConfig{
		Name:       "Person",
		Schema:     schema,
		ReadFilter: rf,
	})
	checkErr(t, err)
	if !bytes.Equal(c.GetReadFilter(), []byte(rf)) {
		t.Fatal("got read filter does not match original")
	}
}

func TestGetIndexes(t *testing.T) {
	t.Parallel()
	db, clean := createTestDB(t)
	defer clean()
	schema := util.SchemaFromInstance(&Person2{}, false)
	c, err := db.NewCollection(CollectionConfig{
		Name:   "Person",
		Schema: schema,
	})
	checkErr(t, err)
	indexes := c.GetIndexes()
	if len(indexes) != 0 {
		t.Fatalf("expected %d indexes, got %d", 0, len(indexes))
	}
	err = c.addIndex(schema, Index{Path: "Name", Unique: false})
	checkErr(t, err)
	indexes = c.GetIndexes()
	if len(indexes) != 1 {
		t.Fatalf("expected %d indexes, got %d", 1, len(indexes))
	}
	if indexes[0].Unique {
		t.Fatal("index on Name should not be unique")
	}
}

func TestDropIndex(t *testing.T) {
	t.Parallel()
	db, clean := createTestDB(t)
	defer clean()
	schema := util.SchemaFromInstance(&Person2{}, false)
	c, err := db.NewCollection(CollectionConfig{
		Name:   "Person",
		Schema: schema,
	})
	checkErr(t, err)
	err = c.addIndex(schema, Index{Path: "Name", Unique: true})
	checkErr(t, err)
	err = c.addIndex(schema, Index{Path: "Age", Unique: false})
	checkErr(t, err)
	err = c.addIndex(schema, Index{Path: "Toys.Favorite", Unique: false})
	checkErr(t, err)

	t.Run("DropIndex", func(t *testing.T) {
		err := c.dropIndex("Age")
		checkErr(t, err)
		indexes := c.GetIndexes()
		if len(indexes) != 2 {
			t.Fatalf("expected %d indexes, got %d", 2, len(indexes))
		}
	})
	t.Run("DropNestedIndex", func(t *testing.T) {
		err := c.dropIndex("Toys.Favorite")
		checkErr(t, err)
		indexes := c.GetIndexes()
		if len(indexes) != 1 {
			t.Fatalf("expected %d indexes, got %d", 1, len(indexes))
		}
	})
}

func TestCreateInstance(t *testing.T) {
	t.Parallel()
	t.Run("Single", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		c, err := db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}, false),
		})
		checkErr(t, err)

		t.Run("WithImplicitTx", func(t *testing.T) {
			newPerson := util.JSONFromInstance(Person{Name: "Foo", Age: 42})
			res, err := c.Create(newPerson)
			checkErr(t, err)
			newPerson = util.SetJSONID(res, newPerson)
			assertPersonInCollection(t, c, newPerson)
		})
		t.Run("WithTx", func(t *testing.T) {
			newPerson := util.JSONFromInstance(Person{Name: "Foo", Age: 42})
			var res []core.InstanceID
			err = c.WriteTxn(func(txn *Txn) (err error) {
				res, err = txn.Create(newPerson)
				return
			})
			checkErr(t, err)
			newPerson = util.SetJSONID(res[0], newPerson)
			assertPersonInCollection(t, c, newPerson)
		})
	})
	t.Run("Multiple", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		c, err := db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}, false),
		})
		checkErr(t, err)

		newPerson1 := util.JSONFromInstance(Person{Name: "Foo1", Age: 42})
		newPerson2 := util.JSONFromInstance(Person{Name: "Foo2", Age: 43})
		var res1 []core.InstanceID
		var res2 []core.InstanceID
		err = c.WriteTxn(func(txn *Txn) (err error) {
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
		assertPersonInCollection(t, c, newPerson1)
		assertPersonInCollection(t, c, newPerson2)
	})
	t.Run("WithDefinedID", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		c, err := db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}, false),
		})
		checkErr(t, err)

		definedID := core.NewInstanceID()
		newPerson := util.JSONFromInstance(&Person{ID: definedID, Name: "Foo1", Age: 42})
		_, err = c.Create(newPerson)
		checkErr(t, err)

		exists, err := c.Has(definedID)
		checkErr(t, err)
		if !exists {
			t.Fatal("manually defined instance ID should exist")
		}
		assertPersonInCollection(t, c, newPerson)
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
		p2 := util.JSONFromInstance(Person{ID: res, Name: "Fool2", Age: 43})
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
		p = util.SetJSONID(res, p)
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
			return txn.Delete(res)
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
	c, err := db.NewCollection(CollectionConfig{
		Name:   "Person",
		Schema: util.SchemaFromInstance(&Person{}, false),
	})
	checkErr(t, err)

	newPerson := util.JSONFromInstance(Person{Name: "Foo", Age: 42})
	var res []core.InstanceID
	err = c.WriteTxn(func(txn *Txn) (err error) {
		res, err = txn.Create(newPerson)
		return
	})
	checkErr(t, err)
	newPerson = util.SetJSONID(res[0], newPerson)
	newPersonInstance := &Person{}
	util.InstanceFromJSON(newPerson, newPersonInstance)

	t.Run("WithImplicitTx", func(t *testing.T) {
		found, err := c.FindByID(res[0])
		checkErr(t, err)

		foundInstance := &Person{}
		util.InstanceFromJSON(found, foundInstance)

		if !reflect.DeepEqual(newPersonInstance, foundInstance) {
			t.Fatalf(errInvalidInstanceState)
		}
	})
	t.Run("WithReadTx", func(t *testing.T) {
		var found []byte
		err = c.ReadTxn(func(txn *Txn) (err error) {
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
		err = c.WriteTxn(func(txn *Txn) (err error) {
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

func TestVerifyInstance(t *testing.T) {
	t.Parallel()
	t.Run("WithoutWriteValidator", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		c, err := db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}, false),
		})
		checkErr(t, err)

		newPerson := util.JSONFromInstance(Person{Name: "Alice", Age: 42})
		var res []core.InstanceID
		err = c.WriteTxn(func(txn *Txn) (err error) {
			res, err = txn.Create(newPerson)
			return
		})
		checkErr(t, err)

		newPerson = util.JSONFromInstance(Person{ID: res[0], Name: "Alice", Age: 43})
		err = c.WriteTxn(func(txn *Txn) (err error) {
			err = txn.Verify(newPerson)
			return
		})
		checkErr(t, err)
	})
	t.Run("WithWriteValidator", func(t *testing.T) {
		t.Parallel()
		db, clean := createTestDB(t)
		defer clean()
		c, err := db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}, false),
			WriteValidator: `
				var type = event.patch.type
				var patch = event.patch.json_patch
				switch (type) {
				  case "delete":
				    return false
				  default:
				    if (patch.Age > 50) {
				      return false
				    }
				    return true
				}
			`,
		})
		checkErr(t, err)

		newPerson := util.JSONFromInstance(Person{Name: "Alice", Age: 42})
		var res []core.InstanceID
		err = c.WriteTxn(func(txn *Txn) (err error) {
			res, err = txn.Create(newPerson)
			return
		})
		checkErr(t, err)

		newPerson = util.JSONFromInstance(Person{ID: res[0], Name: "Alice", Age: 51})
		err = c.WriteTxn(func(txn *Txn) (err error) {
			err = txn.Verify(newPerson)
			return
		})
		if err == nil {
			t.Fatal("write should have been invalid")
		}

		newPerson = util.JSONFromInstance(Person{ID: res[0], Name: "Alice", Age: 43})
		err = c.WriteTxn(func(txn *Txn) (err error) {
			err = txn.Verify(newPerson)
			return
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
		c, err := db.NewCollection(CollectionConfig{
			Name:   "Person",
			Schema: util.SchemaFromInstance(&Person{}, false),
		})
		checkErr(t, err)

		newPerson := util.JSONFromInstance(Person{Name: "Alice", Age: 42})
		var res []core.InstanceID
		err = c.WriteTxn(func(txn *Txn) (err error) {
			res, err = txn.Create(newPerson)
			return
		})
		checkErr(t, err)

		err = c.WriteTxn(func(txn *Txn) error {
			instance, err := txn.FindByID(res[0])
			checkErr(t, err)

			p := &Person{}
			util.InstanceFromJSON(instance, p)

			p.Name = "Bob"
			return txn.Save(util.JSONFromInstance(p))
		})
		checkErr(t, err)

		instance, err := c.FindByID(res[0])
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
	c, err := db.NewCollection(CollectionConfig{
		Name:   "Person",
		Schema: util.SchemaFromInstance(&Person{}, false),
	})
	checkErr(t, err)

	newPerson := util.JSONFromInstance(Person{Name: "Alice", Age: 42})
	var res []core.InstanceID
	err = c.WriteTxn(func(txn *Txn) (err error) {
		res, err = txn.Create(newPerson)
		return
	})
	checkErr(t, err)

	err = c.Delete(res[0])
	checkErr(t, err)

	_, err = c.FindByID(res[0])
	if err != ErrInstanceNotFound {
		t.Fatalf("FindByID: instance shouldn't exist")
	}
	if exist, err := c.Has(res[0]); exist || err != nil {
		t.Fatalf("Has: instance shouldn't exist")
	}

	// Try to delete again
	if err = c.Delete(res[0]); err != ErrInstanceNotFound {
		t.Fatalf("cant't delete non-existent instance")
	}
}

type PersonFake struct {
	ID   core.InstanceID `json:"_id"`
	Name string
}

func TestInvalidActions(t *testing.T) {
	t.Parallel()
	db, clean := createTestDB(t)
	defer clean()
	c, err := db.NewCollection(CollectionConfig{
		Name:   "Person",
		Schema: util.SchemaFromInstance(&Person{}, false),
	})
	checkErr(t, err)
	t.Run("Create", func(t *testing.T) {
		f := util.JSONFromInstance(PersonFake{Name: "fake"})
		if _, err := c.Create(f); !errors.Is(err, ErrInvalidSchemaInstance) {
			t.Fatalf("instance should be invalid compared to schema, got: %v", err)
		}
	})
	t.Run("Save", func(t *testing.T) {
		r := util.JSONFromInstance(Person{Name: "real"})
		_, err := c.Create(r)
		checkErr(t, err)
		f := util.JSONFromInstance(PersonFake{Name: "fake"})
		if err := c.Save(f); !errors.Is(err, ErrInvalidSchemaInstance) {
			t.Fatalf("instance should be invalid compared to schema, got: %v", err)
		}
	})
}

func assertPersonInCollection(t *testing.T, c *Collection, personBytes []byte) {
	t.Helper()
	person := &Person{}
	util.InstanceFromJSON(personBytes, person)
	res, err := c.FindByID(person.ID)
	checkErr(t, err)
	p := &Person{}
	util.InstanceFromJSON(res, p)
	if !reflect.DeepEqual(person, p) {
		t.Fatalf(errInvalidInstanceState)
	}
}
