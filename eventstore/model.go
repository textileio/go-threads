package eventstore

import (
	"encoding/json"
	"errors"
	"reflect"

	"github.com/alecthomas/jsonschema"
	ds "github.com/ipfs/go-datastore"
	"github.com/textileio/go-textile-threads/core"
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

	baseKey = ds.NewKey("/model/")
)

// Model contains instances of a schema, and provides operations
// for creating, updating, deleting, and quering them.
type Model struct {
	schema       *jsonschema.Schema
	schemaLoader gojsonschema.JSONLoader
	valueType    reflect.Type
	datastore    ds.Datastore
	eventcodec   core.EventCodec
	dispatcher   *Dispatcher
	dsKey        ds.Key
	store        *Store
}

func newModel(name string, defaultInstance interface{}, datastore ds.Datastore, dispatcher *Dispatcher, eventcreator core.EventCodec, s *Store) *Model {
	schema := jsonschema.Reflect(defaultInstance)
	schemaLoader := gojsonschema.NewGoLoader(schema)
	m := &Model{
		schema:       schema,
		schemaLoader: schemaLoader,
		datastore:    datastore,
		valueType:    reflect.TypeOf(defaultInstance),
		dispatcher:   dispatcher,
		eventcodec:   eventcreator,
		dsKey:        baseKey.ChildString(name),
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
	m.ReadTxn(func(txn *Txn) error {
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

// Reduce processes an event into the model.
func (m *Model) Reduce(event core.Event) error {
	log.Debugf("reducer %s start", m.schema.Ref)
	if event.Type() != m.schema.Ref {
		log.Debugf("ignoring event from uninteresting type")
		return nil
	}

	return m.eventcodec.Reduce(event, m.datastore, m.dsKey)
}

func (m *Model) validInstance(v interface{}) (bool, error) {
	vLoader := gojsonschema.NewGoLoader(v)
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

		id := getEntityID(new[i])
		if id == core.EmptyEntityID {
			id = setNewEntityID(new[i])
		}
		key := t.model.dsKey.ChildString(id.String())
		exists, err := t.model.datastore.Has(key)
		if err != nil {
			return err
		}
		if exists {
			return errCantCreateExistingInstance
		}

		a := core.Action{
			Type:       core.Create,
			EntityID:   id,
			EntityType: t.model.schema.Ref,
			Previous:   nil,
			Current:    new[i],
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

		id := getEntityID(updated[i])
		key := t.model.dsKey.ChildString(id.String())
		beforeBytes, err := t.model.datastore.Get(key)
		if err == ds.ErrNotFound {
			return errCantSaveNonExistentInstance
		}
		if err != nil {
			return err
		}

		before := reflect.New(t.model.valueType.Elem()).Interface()
		err = json.Unmarshal(beforeBytes, before)
		if err != nil {
			return err
		}
		a := core.Action{
			Type:       core.Save,
			EntityID:   id,
			EntityType: t.model.schema.Ref,
			Previous:   before,
			Current:    updated[i],
		}
		t.actions = append(t.actions, a)
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
		key := t.model.dsKey.ChildString(ids[i].String())
		exists, err := t.model.datastore.Has(key)
		if err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		a := core.Action{
			Type:       core.Delete,
			EntityID:   ids[i],
			EntityType: t.model.schema.Ref,
			Previous:   nil,
			Current:    nil,
		}
		t.actions = append(t.actions, a)
	}
	return nil
}

// Has returns true if all IDs exists in the model, false
// otherwise.
func (t *Txn) Has(ids ...core.EntityID) (bool, error) {
	for i := range ids {
		key := t.model.dsKey.ChildString(ids[i].String())
		exists, err := t.model.datastore.Has(key)
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
	key := t.model.dsKey.ChildString(id.String())
	bytes, err := t.model.datastore.Get(key)
	if errors.Is(err, ds.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
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

	events, err := t.model.eventcodec.Create(t.actions)
	if err != nil {
		return err
	}

	for _, e := range events {
		if err := t.model.dispatcher.Dispatch(e); err != nil {
			return err
		}
	}
	return nil
}

// Discard discards all changes done in the current
// transaction.
func (t *Txn) Discard() {
	t.discarded = true
}

func getEntityID(t interface{}) core.EntityID {
	v := reflect.ValueOf(t)
	if v.Type().Kind() != reflect.Ptr {
		v = reflect.New(reflect.TypeOf(v))
	}
	v = v.Elem().FieldByName(idFieldName)
	if !v.IsValid() || v.Type() != reflect.TypeOf(core.EmptyEntityID) {
		panic("invalid instance: doesn't have EntityID attribute")
	}
	return core.EntityID(v.String())
}

func setNewEntityID(t interface{}) core.EntityID {
	v := reflect.ValueOf(t)
	if v.Type().Kind() != reflect.Ptr {
		v = reflect.New(reflect.TypeOf(v))
	}
	newID := core.NewEntityID()
	v.Elem().FieldByName(idFieldName).Set(reflect.ValueOf(newID))
	return newID
}
