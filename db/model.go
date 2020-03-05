package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/alecthomas/jsonschema"
	jsonpatch "github.com/evanphx/json-patch"
	ds "github.com/ipfs/go-datastore"
	core "github.com/textileio/go-threads/core/store"
	"github.com/tidwall/gjson"
	"github.com/xeipuuv/gojsonschema"
)

var (
	// ErrNotFound indicates that the specified instance doesn't
	// exist in the model.
	ErrNotFound = errors.New("instance not found")
	// ErrReadonlyTx indicates that no write operations can be done since
	// the current transaction is readonly.
	ErrReadonlyTx = errors.New("read only transaction")
	// ErrInvalidSchemaInstance indicates the current operation is from an
	// instance that doesn't satisfy the model schema.
	ErrInvalidSchemaInstance = errors.New("instance doesn't correspond to schema")

	errAlreadyDiscardedCommitedTxn = errors.New("can't commit discarded/commited txn")
	errCantCreateExistingInstance  = errors.New("can't create already existing instance")
	errCantSaveNonExistentInstance = errors.New("can't save unkown instance")

	baseKey = dsStorePrefix.ChildString("model")
)

// Model contains instances of a schema, and provides operations
// for creating, updating, deleting, and quering them.
type Model struct {
	name         string
	schemaLoader gojsonschema.JSONLoader
	valueType    reflect.Type
	db           *DB
	indexes      map[string]Index
}

func newModel(name string, defaultInstance interface{}, d *DB) *Model {
	schema := jsonschema.Reflect(defaultInstance)
	schemaLoader := gojsonschema.NewGoLoader(schema)
	m := &Model{
		name:         name,
		schemaLoader: schemaLoader,
		valueType:    reflect.TypeOf(defaultInstance),
		db:           d,
		indexes:      make(map[string]Index),
	}
	return m
}

func newModelFromSchema(name string, schema string, d *DB) *Model {
	schemaLoader := gojsonschema.NewStringLoader(schema)
	m := &Model{
		name:         name,
		schemaLoader: schemaLoader,
		valueType:    nil,
		db:           d,
		indexes:      make(map[string]Index),
	}
	return m
}

func (m *Model) BaseKey() ds.Key {
	return baseKey.ChildString(m.name)
}

// Indexes is a map of model properties to Indexes
func (m *Model) Indexes() map[string]Index {
	return m.indexes
}

// AddIndex creates a new index based on the given path string.
// Set unique to true if you want a unique constraint on the given path.
// See https://github.com/tidwall/gjson for documentation on the supported path structure.
// Adding an index will override any overlapping index values if they already exist.
// @note: This does NOT currently build the index. If items have been added prior to adding
// a new index, they will NOT be indexed a posteriori.
func (m *Model) AddIndex(path string, unique bool) error {
	m.indexes[path] = Index{
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
// that tries to mutate an instance of the model will ErrReadonlyTx.
// Provides serializable isolation gurantees.
func (m *Model) ReadTxn(f func(txn *Txn) error) error {
	return m.db.readTxn(m, f)
}

// WriteTxn creates an explicit write transaction. Provides
// serializable isolation gurantees.
func (m *Model) WriteTxn(f func(txn *Txn) error) error {
	return m.db.writeTxn(m, f)
}

// FindByID finds an instance by its ID and saves it in v.
// If doesn't exists returns ErrNotFound.
func (m *Model) FindByID(id core.EntityID, v interface{}) error {
	return m.ReadTxn(func(txn *Txn) error {
		return txn.FindByID(id, v)
	})
}

// Create creates instances in the model.
func (m *Model) Create(vs ...interface{}) error {
	return m.WriteTxn(func(txn *Txn) error {
		return txn.Create(vs...)
	})
}

// Delete deletes instances by its IDs. It doesn't
// fail if the ID doesn't exist.
func (m *Model) Delete(ids ...core.EntityID) error {
	return m.WriteTxn(func(txn *Txn) error {
		return txn.Delete(ids...)
	})
}

// Save saves changes of instances in the model.
func (m *Model) Save(vs ...interface{}) error {
	return m.WriteTxn(func(txn *Txn) error {
		return txn.Save(vs...)
	})
}

// Has returns true if all IDs exist in the model, false
// otherwise.
func (m *Model) Has(ids ...core.EntityID) (exists bool, err error) {
	_ = m.ReadTxn(func(txn *Txn) error {
		exists, err = txn.Has(ids...)
		return err
	})
	return
}

// Find executes a Query into result.
func (m *Model) Find(result interface{}, q *Query) error {
	return m.ReadTxn(func(txn *Txn) error {
		return txn.Find(result, q)
	})
}

// FindJSON executes a Query in in JSONMode and returns the result.
func (m *Model) FindJSON(q *JSONQuery) (ret []string, err error) {
	_ = m.ReadTxn(func(txn *Txn) error {
		ret, err = txn.FindJSON(q)
		return err
	})
	return
}

func (m *Model) validInstance(v interface{}) (bool, error) {
	var vLoader gojsonschema.JSONLoader
	if m.db.jsonMode {
		strJSON := v.(*string)
		vLoader = gojsonschema.NewBytesLoader([]byte(*strJSON))
	} else {
		vLoader = gojsonschema.NewGoLoader(v)
	}
	r, err := gojsonschema.Validate(m.schemaLoader, vLoader)
	if err != nil {
		return false, err
	}

	return r.Valid(), nil
}

// Sanity check
var _ Indexer = (*Model)(nil)

// Txn represents a read/write transaction in the db. It allows for
// serializable isolation level within the db.
type Txn struct {
	model     *Model
	discarded bool
	commited  bool
	readonly  bool

	actions []core.Action
}

// Create creates new instances in the model
// If the ID value on the instance is nil or otherwise a null value (e.g., "" in jsonMode),
// the ID is updated in-place to reflect the automatically-genereted UUID.
func (t *Txn) Create(new ...interface{}) error {
	for i := range new {
		if t.readonly {
			return ErrReadonlyTx
		}
		valid, err := t.model.validInstance(new[i])
		if err != nil {
			return err
		}
		if !valid {
			return ErrInvalidSchemaInstance
		}

		jsonMode := t.model.db.jsonMode
		id := getEntityID(new[i], jsonMode)
		if id == core.EmptyEntityID {
			id = setNewEntityID(new[i], jsonMode)
		}
		key := baseKey.ChildString(t.model.name).ChildString(id.String())
		exists, err := t.model.db.datastore.Has(key)
		if err != nil {
			return err
		}
		if exists {
			return errCantCreateExistingInstance
		}

		a := core.Action{
			Type:      core.Create,
			EntityID:  id,
			ModelName: t.model.name,
			Previous:  nil,
			Current:   new[i],
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

		valid, err := t.model.validInstance(updated[i])
		if err != nil {
			return err
		}
		if !valid {
			return ErrInvalidSchemaInstance
		}

		id := getEntityID(updated[i], t.model.db.jsonMode)
		key := baseKey.ChildString(t.model.name).ChildString(id.String())
		beforeBytes, err := t.model.db.datastore.Get(key)
		if err == ds.ErrNotFound {
			return errCantSaveNonExistentInstance
		}
		if err != nil {
			return err
		}

		var previous interface{}
		previous = beforeBytes
		if !t.model.db.jsonMode {
			before := reflect.New(t.model.valueType.Elem()).Interface()
			if err = json.Unmarshal(beforeBytes, before); err != nil {
				return err
			}
			previous = before
		}
		t.actions = append(t.actions, core.Action{
			Type:      core.Save,
			EntityID:  id,
			ModelName: t.model.name,
			Previous:  previous,
			Current:   updated[i],
		})
	}
	return nil
}

// Delete deletes instances by ID when the current
// transaction commits.
func (t *Txn) Delete(ids ...core.EntityID) error {
	for i := range ids {
		if t.readonly {
			return ErrReadonlyTx
		}
		key := baseKey.ChildString(t.model.name).ChildString(ids[i].String())
		exists, err := t.model.db.datastore.Has(key)
		if err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		a := core.Action{
			Type:      core.Delete,
			EntityID:  ids[i],
			ModelName: t.model.name,
			Previous:  nil,
			Current:   nil,
		}
		t.actions = append(t.actions, a)
	}
	return nil
}

// Has returns true if all IDs exists in the model, false
// otherwise.
func (t *Txn) Has(ids ...core.EntityID) (bool, error) {
	for i := range ids {
		key := baseKey.ChildString(t.model.name).ChildString(ids[i].String())
		exists, err := t.model.db.datastore.Has(key)
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
func (t *Txn) FindByID(id core.EntityID, v interface{}) error {
	key := baseKey.ChildString(t.model.name).ChildString(id.String())
	bytes, err := t.model.db.datastore.Get(key)
	if errors.Is(err, ds.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if t.model.db.jsonMode {
		str := string(bytes)
		rflStr := reflect.ValueOf(str)
		reflV := reflect.ValueOf(v)
		reflV.Elem().Set(rflStr)

		return nil
	}
	return json.Unmarshal(bytes, v)
}

// Commit applies all changes done in the current transaction
// to the model. This is a syncrhonous call so changes can
// be assumed to be applied on function return.
func (t *Txn) Commit() error {
	if t.discarded || t.commited {
		return errAlreadyDiscardedCommitedTxn
	}
	events, node, err := t.model.db.eventcodec.Create(t.actions)
	if err != nil {
		return err
	}
	if err := t.model.db.dispatcher.Dispatch(events); err != nil {
		return err
	}
	if err := t.model.db.notifyTxnEvents(node); err != nil {
		return err
	}
	return nil
}

// Discard discards all changes done in the current
// transaction.
func (t *Txn) Discard() {
	t.discarded = true
}

func getEntityID(t interface{}, jsonMode bool) core.EntityID {
	if jsonMode {
		partial := &struct{ ID *string }{}
		if err := json.Unmarshal([]byte(*(t.(*string))), partial); err != nil {
			log.Fatalf("error when unmarshaling json instance: %v", err)
		}
		if partial.ID == nil {
			log.Fatal("invalid instance: doesn't have an ID attribute")
		}
		if *partial.ID != "" && !core.IsValidEntityID(*partial.ID) {
			log.Fatal("invalid instance: invalid ID value")
		}
		return core.EntityID(*partial.ID)
	} else {
		v := reflect.ValueOf(t)
		if v.Type().Kind() != reflect.Ptr {
			v = reflect.New(reflect.TypeOf(v))
		}
		v = v.Elem().FieldByName(idFieldName)
		if !v.IsValid() || v.Type() != reflect.TypeOf(core.EmptyEntityID) {
			log.Fatal("invalid instance: doesn't have EntityID attribute")
		}
		return core.EntityID(v.String())
	}
}

func setNewEntityID(t interface{}, jsonMode bool) core.EntityID {
	newID := core.NewEntityID()
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
