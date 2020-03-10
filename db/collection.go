package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/alecthomas/jsonschema"
	jsonpatch "github.com/evanphx/json-patch"
	ds "github.com/ipfs/go-datastore"
	core "github.com/textileio/go-threads/core/db"
	"github.com/tidwall/gjson"
	"github.com/xeipuuv/gojsonschema"
)

var (
	// ErrNotFound indicates that the specified instance doesn't
	// exist in the collection.
	ErrNotFound = errors.New("instance not found")
	// ErrReadonlyTx indicates that no write operations can be done since
	// the current transaction is readonly.
	ErrReadonlyTx = errors.New("read only transaction")
	// ErrInvalidSchemaInstance indicates the current operation is from an
	// instance that doesn't satisfy the collection schema.
	ErrInvalidSchemaInstance = errors.New("instance doesn't correspond to schema")

	errAlreadyDiscardedCommitedTxn = errors.New("can't commit discarded/commited txn")
	errCantCreateExistingInstance  = errors.New("can't create already existing instance")
	errCantSaveNonExistentInstance = errors.New("can't save unkown instance")

	baseKey = dsDBPrefix.ChildString("collection")
)

// Collection contains instances of a schema, and provides operations
// for creating, updating, deleting, and quering them.
type Collection struct {
	name         string
	schemaLoader gojsonschema.JSONLoader
	valueType    reflect.Type
	db           *DB
	indexes      map[string]Index
}

func newCollection(name string, defaultInstance interface{}, d *DB) *Collection {
	schema := jsonschema.Reflect(defaultInstance)
	schemaLoader := gojsonschema.NewGoLoader(schema)
	c := &Collection{
		name:         name,
		schemaLoader: schemaLoader,
		valueType:    reflect.TypeOf(defaultInstance),
		db:           d,
		indexes:      make(map[string]Index),
	}
	return c
}

func newCollectionFromSchema(name string, schema string, d *DB) *Collection {
	schemaLoader := gojsonschema.NewStringLoader(schema)
	c := &Collection{
		name:         name,
		schemaLoader: schemaLoader,
		valueType:    nil,
		db:           d,
		indexes:      make(map[string]Index),
	}
	return c
}

func (c *Collection) BaseKey() ds.Key {
	return baseKey.ChildString(c.name)
}

// Indexes is a map of collection properties to Indexes
func (c *Collection) Indexes() map[string]Index {
	return c.indexes
}

// AddIndex creates a new index based on the given path string.
// Set unique to true if you want a unique constraint on the given path.
// See https://github.com/tidwall/gjson for documentation on the supported path structure.
// Adding an index will override any overlapping index values if they already exist.
// @note: This does NOT currently build the index. If items have been added prior to adding
// a new index, they will NOT be indexed a posteriori.
func (c *Collection) AddIndex(path string, unique bool) error {
	c.indexes[path] = Index{
		IndexFunc: func(field string, value []byte) (ds.Key, error) {
			result := gjson.GetBytes(value, field)
			if !result.Exists() {
				return ds.Key{}, ErrNotIndexable
			}
			return ds.NewKey(result.String()), nil
		},
		Unique: unique,
	}
	return nil
}

// ReadTxn creates an explicit readonly transaction. Any operation
// that tries to mutate an instance of the collection will ErrReadonlyTx.
// Provides serializable isolation gurantees.
func (c *Collection) ReadTxn(f func(txn *Txn) error) error {
	return c.db.readTxn(c, f)
}

// WriteTxn creates an explicit write transaction. Provides
// serializable isolation gurantees.
func (c *Collection) WriteTxn(f func(txn *Txn) error) error {
	return c.db.writeTxn(c, f)
}

// FindByID finds an instance by its ID and saves it in v.
// If doesn't exists returns ErrNotFound.
func (c *Collection) FindByID(id core.InstanceID, v interface{}) error {
	return c.ReadTxn(func(txn *Txn) error {
		return txn.FindByID(id, v)
	})
}

// Create creates instances in the collection.
func (c *Collection) Create(vs ...interface{}) error {
	return c.WriteTxn(func(txn *Txn) error {
		return txn.Create(vs...)
	})
}

// Delete deletes instances by its IDs. It doesn't
// fail if the ID doesn't exist.
func (c *Collection) Delete(ids ...core.InstanceID) error {
	return c.WriteTxn(func(txn *Txn) error {
		return txn.Delete(ids...)
	})
}

// Save saves changes of instances in the collection.
func (c *Collection) Save(vs ...interface{}) error {
	return c.WriteTxn(func(txn *Txn) error {
		return txn.Save(vs...)
	})
}

// Has returns true if all IDs exist in the collection, false
// otherwise.
func (c *Collection) Has(ids ...core.InstanceID) (exists bool, err error) {
	_ = c.ReadTxn(func(txn *Txn) error {
		exists, err = txn.Has(ids...)
		return err
	})
	return
}

// Find executes a Query into result.
func (c *Collection) Find(result interface{}, q *Query) error {
	return c.ReadTxn(func(txn *Txn) error {
		return txn.Find(result, q)
	})
}

// FindJSON executes a Query in in JSONMode and returns the result.
func (c *Collection) FindJSON(q *JSONQuery) (ret []string, err error) {
	_ = c.ReadTxn(func(txn *Txn) error {
		ret, err = txn.FindJSON(q)
		return err
	})
	return
}

func (c *Collection) validInstance(v interface{}) (bool, error) {
	var vLoader gojsonschema.JSONLoader
	if c.db.jsonMode {
		strJSON := v.(*string)
		vLoader = gojsonschema.NewBytesLoader([]byte(*strJSON))
	} else {
		vLoader = gojsonschema.NewGoLoader(v)
	}
	r, err := gojsonschema.Validate(c.schemaLoader, vLoader)
	if err != nil {
		return false, err
	}

	return r.Valid(), nil
}

// Sanity check
var _ Indexer = (*Collection)(nil)

// Txn represents a read/write transaction in the db. It allows for
// serializable isolation level within the db.
type Txn struct {
	collection *Collection
	discarded  bool
	commited   bool
	readonly   bool

	actions []core.Action
}

// Create creates new instances in the collection
// If the ID value on the instance is nil or otherwise a null value (e.g., "" in jsonMode),
// the ID is updated in-place to reflect the automatically-genereted UUID.
func (t *Txn) Create(new ...interface{}) error {
	for i := range new {
		if t.readonly {
			return ErrReadonlyTx
		}
		valid, err := t.collection.validInstance(new[i])
		if err != nil {
			return err
		}
		if !valid {
			return ErrInvalidSchemaInstance
		}

		jsonMode := t.collection.db.jsonMode
		id := getInstanceID(new[i], jsonMode)
		if id == core.EmptyInstanceID {
			id = setNewInstanceID(new[i], jsonMode)
		}
		key := baseKey.ChildString(t.collection.name).ChildString(id.String())
		exists, err := t.collection.db.datastore.Has(key)
		if err != nil {
			return err
		}
		if exists {
			return errCantCreateExistingInstance
		}

		a := core.Action{
			Type:           core.Create,
			InstanceID:     id,
			CollectionName: t.collection.name,
			Previous:       nil,
			Current:        new[i],
		}
		t.actions = append(t.actions, a)
	}
	return nil
}

// Save saves an instance changes to be commited when the
// current transaction commits.
func (t *Txn) Save(updated ...interface{}) error {
	for i := range updated {
		if t.readonly {
			return ErrReadonlyTx
		}

		valid, err := t.collection.validInstance(updated[i])
		if err != nil {
			return err
		}
		if !valid {
			return ErrInvalidSchemaInstance
		}

		id := getInstanceID(updated[i], t.collection.db.jsonMode)
		key := baseKey.ChildString(t.collection.name).ChildString(id.String())
		beforeBytes, err := t.collection.db.datastore.Get(key)
		if err == ds.ErrNotFound {
			return errCantSaveNonExistentInstance
		}
		if err != nil {
			return err
		}

		var previous interface{}
		previous = beforeBytes
		if !t.collection.db.jsonMode {
			before := reflect.New(t.collection.valueType.Elem()).Interface()
			if err = json.Unmarshal(beforeBytes, before); err != nil {
				return err
			}
			previous = before
		}
		t.actions = append(t.actions, core.Action{
			Type:           core.Save,
			InstanceID:     id,
			CollectionName: t.collection.name,
			Previous:       previous,
			Current:        updated[i],
		})
	}
	return nil
}

// Delete deletes instances by ID when the current
// transaction commits.
func (t *Txn) Delete(ids ...core.InstanceID) error {
	for i := range ids {
		if t.readonly {
			return ErrReadonlyTx
		}
		key := baseKey.ChildString(t.collection.name).ChildString(ids[i].String())
		exists, err := t.collection.db.datastore.Has(key)
		if err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		a := core.Action{
			Type:           core.Delete,
			InstanceID:     ids[i],
			CollectionName: t.collection.name,
			Previous:       nil,
			Current:        nil,
		}
		t.actions = append(t.actions, a)
	}
	return nil
}

// Has returns true if all IDs exists in the collection, false
// otherwise.
func (t *Txn) Has(ids ...core.InstanceID) (bool, error) {
	for i := range ids {
		key := baseKey.ChildString(t.collection.name).ChildString(ids[i].String())
		exists, err := t.collection.db.datastore.Has(key)
		if err != nil {
			return false, err
		}
		if !exists {
			return false, nil
		}
	}
	return true, nil
}

// FindByID gets an instance by ID in the current txn scope.
func (t *Txn) FindByID(id core.InstanceID, v interface{}) error {
	key := baseKey.ChildString(t.collection.name).ChildString(id.String())
	bytes, err := t.collection.db.datastore.Get(key)
	if errors.Is(err, ds.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if t.collection.db.jsonMode {
		str := string(bytes)
		rflStr := reflect.ValueOf(str)
		reflV := reflect.ValueOf(v)
		reflV.Elem().Set(rflStr)

		return nil
	}
	return json.Unmarshal(bytes, v)
}

// Commit applies all changes done in the current transaction
// to the collection. This is a syncrhonous call so changes can
// be assumed to be applied on function return.
func (t *Txn) Commit() error {
	if t.discarded || t.commited {
		return errAlreadyDiscardedCommitedTxn
	}
	events, node, err := t.collection.db.eventcodec.Create(t.actions)
	if err != nil {
		return err
	}
	if err := t.collection.db.dispatcher.Dispatch(events); err != nil {
		return err
	}
	if err := t.collection.db.notifyTxnEvents(node); err != nil {
		return err
	}
	return nil
}

// Discard discards all changes done in the current
// transaction.
func (t *Txn) Discard() {
	t.discarded = true
}

func getInstanceID(t interface{}, jsonMode bool) core.InstanceID {
	if jsonMode {
		partial := &struct{ ID *string }{}
		if err := json.Unmarshal([]byte(*(t.(*string))), partial); err != nil {
			log.Fatalf("error when unmarshaling json instance: %v", err)
		}
		if partial.ID == nil {
			log.Fatal("invalid instance: doesn't have an ID attribute")
		}
		if *partial.ID != "" && !core.IsValidInstanceID(*partial.ID) {
			log.Fatal("invalid instance: invalid ID value")
		}
		return core.InstanceID(*partial.ID)
	} else {
		v := reflect.ValueOf(t)
		if v.Type().Kind() != reflect.Ptr {
			v = reflect.New(reflect.TypeOf(v))
		}
		v = v.Elem().FieldByName(idFieldName)
		if !v.IsValid() || v.Type() != reflect.TypeOf(core.EmptyInstanceID) {
			log.Fatal("invalid instance: doesn't have InstanceID attribute")
		}
		return core.InstanceID(v.String())
	}
}

func setNewInstanceID(t interface{}, jsonMode bool) core.InstanceID {
	newID := core.NewInstanceID()
	if jsonMode {
		patchedValue, err := jsonpatch.MergePatch([]byte(*(t.(*string))), []byte(fmt.Sprintf(`{"ID": %q}`, newID.String())))
		if err != nil {
			log.Fatalf("error while automatically patching autogenerated ID: %v", err)
		}
		strPatchedValue := string(patchedValue)
		reflectPatchedValue := reflect.ValueOf(&strPatchedValue)
		reflect.ValueOf(t).Elem().Set(reflectPatchedValue.Elem())
	} else {
		v := reflect.ValueOf(t)
		if v.Type().Kind() != reflect.Ptr {
			v = reflect.New(reflect.TypeOf(v))
		}
		v.Elem().FieldByName(idFieldName).Set(reflect.ValueOf(newID))
	}
	return newID
}
