package eventstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/alecthomas/jsonschema"
	jsonpatch "github.com/evanphx/json-patch"
	ds "github.com/ipfs/go-datastore"
	core "github.com/textileio/go-textile-core/store"
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
	store        *Store
}

func newModel(name string, defaultInstance interface{}, s *Store) *Model {
	schema := jsonschema.Reflect(defaultInstance)
	schemaLoader := gojsonschema.NewGoLoader(schema)
	m := &Model{
		name:         name,
		schemaLoader: schemaLoader,
		valueType:    reflect.TypeOf(defaultInstance),
		store:        s,
	}
	return m
}

func newModelFromSchema(name string, schema string, s *Store) *Model {
	schemaLoader := gojsonschema.NewStringLoader(schema)
	m := &Model{
		name:         name,
		schemaLoader: schemaLoader,
		valueType:    nil,
		store:        s,
	}
	return m
}

// ReadTxn creates an explicit readonly transaction. Any operation
// that tries to mutate an instance of the model will ErrReadonlyTx.
// Provides serializable isolation gurantees.
func (m *Model) ReadTxn(f func(txn *Txn) error) error {
	return m.store.readTxn(m, f)
}

// WriteTxn creates an explicit write transaction. Provides
// serializable isolation gurantees.
func (m *Model) WriteTxn(f func(txn *Txn) error) error {
	return m.store.writeTxn(m, f)
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
	if m.store.jsonMode {
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

// Txn represents a read/write transaction in the Store. It allows for
// serializable isolation level within the store.
type Txn struct {
	model     *Model
	discarded bool
	commited  bool
	readonly  bool

	actions []core.Action
}

// Create creates new instances in the model
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

		jsonMode := t.model.store.jsonMode
		id := getEntityID(new[i], jsonMode)
		if id == core.EmptyEntityID {
			id = setNewEntityID(new[i], jsonMode)
		}
		key := baseKey.ChildString(t.model.name).ChildString(id.String())
		exists, err := t.model.store.datastore.Has(key)
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

		id := getEntityID(updated[i], t.model.store.jsonMode)
		key := baseKey.ChildString(t.model.name).ChildString(id.String())
		beforeBytes, err := t.model.store.datastore.Get(key)
		if err == ds.ErrNotFound {
			return errCantSaveNonExistentInstance
		}
		if err != nil {
			return err
		}

		var previous interface{}
		previous = beforeBytes
		if !t.model.store.jsonMode {
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
		exists, err := t.model.store.datastore.Has(key)
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
		exists, err := t.model.store.datastore.Has(key)
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
	bytes, err := t.model.store.datastore.Get(key)
	if errors.Is(err, ds.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if t.model.store.jsonMode {
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
	events, node, err := t.model.store.eventcodec.Create(t.actions)
	if err != nil {
		return err
	}
	if err := t.model.store.dispatcher.Dispatch(events); err != nil {
		return err
	}
	if err := t.model.store.notifyTxnEvents(node); err != nil {
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
