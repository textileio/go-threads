package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/alecthomas/jsonschema"
	jsonpatch "github.com/evanphx/json-patch"
	ds "github.com/ipfs/go-datastore"
	core "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/thread"
	"github.com/xeipuuv/gojsonschema"
)

var (
	// ErrInvalidCollectionSchemaPath indicates path does not resolve to a schema type.
	ErrInvalidCollectionSchemaPath = errors.New("collection schema does not contain path")
	// ErrCollectionNotFound indicates that the specified collection doesn't exist in the db.
	ErrCollectionNotFound = errors.New("collection not found")
	// ErrCollectionAlreadyRegistered indicates a collection with the given name is already registered.
	ErrCollectionAlreadyRegistered = errors.New("collection already registered")
	// ErrInstanceNotFound indicates that the specified instance doesn't exist in the collection.
	ErrInstanceNotFound = errors.New("instance not found")
	// ErrReadonlyTx indicates that no write operations can be done since
	// the current transaction is readonly.
	ErrReadonlyTx = errors.New("read only transaction")
	// ErrInvalidSchemaInstance indicates the current operation is from an
	// instance that doesn't satisfy the collection schema.
	ErrInvalidSchemaInstance = errors.New("instance doesn't correspond to schema")

	errMissingInstanceID           = errors.New("invalid instance: missing _id attribute")
	errAlreadyDiscardedCommitedTxn = errors.New("can't commit discarded/commited txn")
	errCantCreateExistingInstance  = errors.New("can't create already existing instance")
	errCantSaveNonExistentInstance = errors.New("can't save unkown instance")

	baseKey = dsDBPrefix.ChildString("collection")
)

// Collection is a group of instances sharing a schema.
// Collections are like RDBMS tables. They can only exist in a single database.
type Collection struct {
	name         string
	schemaLoader gojsonschema.JSONLoader
	db           *DB
	indexes      map[string]Index
}

// newCollection returns a new Collection from schema.
func newCollection(name string, schema *jsonschema.Schema, d *DB) (*Collection, error) {
	if name != "" && !collectionNameRx.MatchString(name) {
		return nil, ErrInvalidCollectionName
	}
	idType, err := getSchemaTypeAtPath(schema, idFieldName)
	if err != nil {
		if errors.Is(err, ErrInvalidCollectionSchemaPath) {
			return nil, ErrInvalidCollectionSchema
		} else {
			return nil, err
		}
	}
	if idType.Type != "string" {
		return nil, ErrInvalidCollectionSchema
	}
	sb, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	return &Collection{
		name:         name,
		schemaLoader: gojsonschema.NewBytesLoader(sb),
		db:           d,
		indexes:      make(map[string]Index),
	}, nil
}

// baseKey returns the collections base key.
func (c *Collection) baseKey() ds.Key {
	return baseKey.ChildString(c.name)
}

// ReadTxn creates an explicit readonly transaction. Any operation
// that tries to mutate an instance of the collection will ErrReadonlyTx.
// Provides serializable isolation gurantees.
func (c *Collection) ReadTxn(f func(txn *Txn) error, opts ...TxnOption) error {
	return c.db.readTxn(c, f, opts...)
}

// WriteTxn creates an explicit write transaction. Provides
// serializable isolation gurantees.
func (c *Collection) WriteTxn(f func(txn *Txn) error, opts ...TxnOption) error {
	return c.db.writeTxn(c, f, opts...)
}

// FindByID finds an instance by its ID.
// If doesn't exists returns ErrInstanceNotFound.
func (c *Collection) FindByID(id core.InstanceID, opts ...TxnOption) (instance []byte, err error) {
	_ = c.ReadTxn(func(txn *Txn) error {
		instance, err = txn.FindByID(id)
		return err
	}, opts...)
	return
}

// Create creates an instance in the collection.
func (c *Collection) Create(v []byte, opts ...TxnOption) (id core.InstanceID, err error) {
	_ = c.WriteTxn(func(txn *Txn) error {
		var ids []core.InstanceID
		ids, err = txn.Create(v)
		if err != nil {
			return err
		}
		if len(ids) > 0 {
			id = ids[0]
		}
		return nil
	}, opts...)
	return
}

// CreateMany creates multiple instances in the collection.
func (c *Collection) CreateMany(vs [][]byte, opts ...TxnOption) (ids []core.InstanceID, err error) {
	_ = c.WriteTxn(func(txn *Txn) error {
		ids, err = txn.Create(vs...)
		return err
	}, opts...)
	return
}

// Delete deletes an instance by its ID. It doesn't
// fail if the ID doesn't exist.
func (c *Collection) Delete(id core.InstanceID, opts ...TxnOption) error {
	return c.WriteTxn(func(txn *Txn) error {
		return txn.Delete(id)
	}, opts...)
}

// Delete deletes multiple instances by ID. It doesn't
// fail if one of the IDs don't exist.
func (c *Collection) DeleteMany(ids []core.InstanceID, opts ...TxnOption) error {
	return c.WriteTxn(func(txn *Txn) error {
		return txn.Delete(ids...)
	}, opts...)
}

// Save saves changes of an instance in the collection.
func (c *Collection) Save(v []byte, opts ...TxnOption) error {
	return c.WriteTxn(func(txn *Txn) error {
		return txn.Save(v)
	}, opts...)
}

// SaveMany saves changes of multiple instances in the collection.
func (c *Collection) SaveMany(vs [][]byte, opts ...TxnOption) error {
	return c.WriteTxn(func(txn *Txn) error {
		return txn.Save(vs...)
	}, opts...)
}

// Has returns true if ID exists in the collection, false
// otherwise.
func (c *Collection) Has(id core.InstanceID, opts ...TxnOption) (exists bool, err error) {
	_ = c.ReadTxn(func(txn *Txn) error {
		exists, err = txn.Has(id)
		return err
	}, opts...)
	return
}

// HasMany returns true if all IDs exist in the collection, false
// otherwise.
func (c *Collection) HasMany(ids []core.InstanceID, opts ...TxnOption) (exists bool, err error) {
	_ = c.ReadTxn(func(txn *Txn) error {
		exists, err = txn.Has(ids...)
		return err
	}, opts...)
	return
}

// Find executes a Query and returns the result.
func (c *Collection) Find(q *Query, opts ...TxnOption) (instances [][]byte, err error) {
	_ = c.ReadTxn(func(txn *Txn) error {
		instances, err = txn.Find(q)
		return err
	}, opts...)
	return
}

// Txn represents a read/write transaction in the db. It allows for
// serializable isolation level within the db.
type Txn struct {
	collection *Collection
	token      thread.Token
	discarded  bool
	commited   bool
	readonly   bool

	actions []core.Action
}

// Create creates new instances in the collection
// If the ID value on the instance is nil or otherwise a null value (e.g., ""),
// and ID is generated and used to store the instance.
func (t *Txn) Create(new ...[]byte) ([]core.InstanceID, error) {
	results := make([]core.InstanceID, len(new))
	for i := range new {
		if t.readonly {
			return nil, ErrReadonlyTx
		}

		updated := make([]byte, len(new[i]))
		copy(updated, new[i])

		id, err := getInstanceID(updated)
		if err != nil && !errors.Is(err, errMissingInstanceID) {
			return nil, err
		}
		if id == core.EmptyInstanceID {
			id, updated = setNewInstanceID(updated)
		}

		if err := t.collection.validInstance(updated); err != nil {
			return nil, err
		}

		results[i] = id
		key := baseKey.ChildString(t.collection.name).ChildString(id.String())
		exists, err := t.collection.db.datastore.Has(key)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, errCantCreateExistingInstance
		}

		a := core.Action{
			Type:           core.Create,
			InstanceID:     id,
			CollectionName: t.collection.name,
			Previous:       nil,
			Current:        updated,
		}
		t.actions = append(t.actions, a)
	}
	return results, nil
}

// Save saves an instance changes to be commited when the current transaction commits.
func (t *Txn) Save(updated ...[]byte) error {
	for i := range updated {
		if t.readonly {
			return ErrReadonlyTx
		}

		item := make([]byte, len(updated[i]))
		copy(item, updated[i])

		if err := t.collection.validInstance(item); err != nil {
			return err
		}

		id, err := getInstanceID(item)
		if err != nil {
			return err
		}
		key := baseKey.ChildString(t.collection.name).ChildString(id.String())
		beforeBytes, err := t.collection.db.datastore.Get(key)
		if err == ds.ErrNotFound {
			return errCantSaveNonExistentInstance
		}
		if err != nil {
			return err
		}

		t.actions = append(t.actions, core.Action{
			Type:           core.Save,
			InstanceID:     id,
			CollectionName: t.collection.name,
			Previous:       beforeBytes,
			Current:        item,
		})
	}
	return nil
}

// validInstance validates the json object against the collection schema.
func (c *Collection) validInstance(v []byte) error {
	r, err := gojsonschema.Validate(c.schemaLoader, gojsonschema.NewBytesLoader(v))
	if err != nil {
		return err
	}
	errs := r.Errors()
	if len(errs) == 0 {
		return nil
	}
	var msg string
	for i, e := range errs {
		msg += e.Field() + ": " + e.Description()
		if i != len(errs)-1 {
			msg += "; "
		}
	}
	return fmt.Errorf("%w: %s", ErrInvalidSchemaInstance, msg)
}

// Delete deletes instances by ID when the current transaction commits.
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
			return ErrInstanceNotFound
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

// Has returns true if all IDs exists in the collection, false otherwise.
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
func (t *Txn) FindByID(id core.InstanceID) ([]byte, error) {
	key := baseKey.ChildString(t.collection.name).ChildString(id.String())
	bytes, err := t.collection.db.datastore.Get(key)
	if errors.Is(err, ds.ErrNotFound) {
		return nil, ErrInstanceNotFound
	}
	if err != nil {
		return nil, err
	}
	return bytes, nil
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
	if len(events) == 0 && node == nil {
		return nil
	}
	if len(events) == 0 || node == nil {
		return fmt.Errorf("created events and node must both be nil or not-nil")
	}
	if err := t.collection.db.dispatcher.Dispatch(events); err != nil {
		return err
	}
	return t.collection.db.notifyTxnEvents(node, t.token)
}

// Discard discards all changes done in the current transaction.
func (t *Txn) Discard() {
	t.discarded = true
}

func getSchemaTypeAtPath(schema *jsonschema.Schema, pth string) (*jsonschema.Type, error) {
	parts := strings.Split(pth, ".")
	jt := schema.Type
	for _, n := range parts {
		props, err := getSchemaTypeProperties(jt, schema.Definitions)
		if err != nil {
			return nil, err
		}
		jt = props[n]
		if jt == nil {
			return nil, ErrInvalidCollectionSchemaPath
		}
	}
	return jt, nil
}

func getSchemaTypeProperties(jt *jsonschema.Type, defs jsonschema.Definitions) (map[string]*jsonschema.Type, error) {
	properties := jt.Properties
	if jt.Ref != "" {
		parts := strings.Split(jt.Ref, "/")
		if len(parts) < 1 {
			return nil, ErrInvalidCollectionSchema
		}
		def := defs[parts[len(parts)-1]]
		if def == nil {
			return nil, ErrInvalidCollectionSchema
		}
		properties = def.Properties
	}
	return properties, nil
}

func getInstanceID(t []byte) (core.InstanceID, error) {
	partial := &struct {
		ID *string `json:"_id"`
	}{}
	if err := json.Unmarshal(t, partial); err != nil {
		return core.EmptyInstanceID, fmt.Errorf("error unmarshaling json instance: %v", err)
	}
	if partial.ID == nil {
		return core.EmptyInstanceID, errMissingInstanceID
	}
	return core.InstanceID(*partial.ID), nil
}

func setNewInstanceID(t []byte) (core.InstanceID, []byte) {
	newID := core.NewInstanceID()
	patchedValue, err := jsonpatch.MergePatch(t, []byte(fmt.Sprintf(`{"%s": %q}`, idFieldName, newID.String())))
	if err != nil {
		log.Fatalf("error while automatically patching autogenerated _id: %v", err)
	}
	return newID, patchedValue
}
