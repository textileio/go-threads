package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/alecthomas/jsonschema"
	jsonpatch "github.com/evanphx/json-patch"
	ds "github.com/ipfs/go-datastore"
	core "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/thread"
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

func newCollection(name string, schema *jsonschema.Schema, d *DB) (*Collection, error) {
	// by default, use top level properties to validate ID string property exists
	properties := schema.Properties
	if schema.Ref != "" {
		// the schema specifies a ref to a nested type, use it instead
		parts := strings.Split(schema.Ref, "/")
		if len(parts) < 1 {
			return nil, ErrInvalidCollectionSchema
		}
		typeName := parts[len(parts)-1]
		refDefinition := schema.Definitions[typeName]
		if refDefinition == nil {
			return nil, ErrInvalidCollectionSchema
		}
		properties = refDefinition.Properties
	}
	if !hasIDProperty(properties) {
		return nil, ErrInvalidCollectionSchema
	}

	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	schemaLoader := gojsonschema.NewBytesLoader(schemaBytes)
	c := &Collection{
		name:         name,
		schemaLoader: schemaLoader,
		valueType:    nil,
		db:           d,
		indexes:      make(map[string]Index),
	}
	return c, nil
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
func (c *Collection) AddIndex(config IndexConfig) error {
	indexKey := dsDBIndexes.ChildString(c.name)
	exists, err := c.db.datastore.Has(indexKey)
	if err != nil {
		return err
	}

	indexes := map[string]IndexConfig{}

	if exists {
		indexesBytes, err := c.db.datastore.Get(indexKey)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(indexesBytes, &indexes); err != nil {
			return err
		}
	}

	// if the index being added is for path ID
	if config.Path == idFieldName {
		// and there already is and index on ID
		if _, exists := indexes[idFieldName]; exists {
			// just return gracefully
			return nil
		}
	}

	indexes[config.Path] = config

	indexBytes, err := json.Marshal(indexes)
	if err != nil {
		return err
	}

	if err := c.db.datastore.Put(indexKey, indexBytes); err != nil {
		return err
	}

	c.indexes[config.Path] = Index{
		IndexFunc: func(field string, value []byte) (ds.Key, error) {
			result := gjson.GetBytes(value, field)
			if !result.Exists() {
				return ds.Key{}, ErrNotIndexable
			}
			return ds.NewKey(result.String()), nil
		},
		Unique: config.Unique,
	}
	return nil
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
// If doesn't exists returns ErrNotFound.
func (c *Collection) FindByID(id core.InstanceID, opts ...TxnOption) (instance []byte, err error) {
	_ = c.ReadTxn(func(txn *Txn) error {
		instance, err = txn.FindByID(id)
		return err
	}, opts...)
	return
}

// Create creates an instance in the collection.
func (c *Collection) Create(v []byte, opts ...TxnOption) (ids []core.InstanceID, err error) {
	_ = c.WriteTxn(func(txn *Txn) error {
		ids, err = txn.Create(v)
		return err
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

// validInstance validates the json object against the collection schema
func (c *Collection) validInstance(v []byte) (bool, error) {
	var vLoader gojsonschema.JSONLoader
	vLoader = gojsonschema.NewBytesLoader(v)
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
	collection  *Collection
	credentials thread.Credentials
	discarded   bool
	commited    bool
	readonly    bool

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

		valid, err := t.collection.validInstance(updated)
		if err != nil {
			return nil, err
		}
		if !valid {
			return nil, ErrInvalidSchemaInstance
		}

		id := getInstanceID(updated)
		if id == core.EmptyInstanceID {
			id, updated = setNewInstanceID(updated)
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

// Save saves an instance changes to be commited when the
// current transaction commits.
func (t *Txn) Save(updated ...[]byte) error {
	for i := range updated {
		if t.readonly {
			return ErrReadonlyTx
		}

		item := make([]byte, len(updated[i]))
		copy(item, updated[i])

		valid, err := t.collection.validInstance(item)
		if err != nil {
			return err
		}
		if !valid {
			return ErrInvalidSchemaInstance
		}

		id := getInstanceID(item)
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
func (t *Txn) FindByID(id core.InstanceID) ([]byte, error) {
	key := baseKey.ChildString(t.collection.name).ChildString(id.String())
	bytes, err := t.collection.db.datastore.Get(key)
	if errors.Is(err, ds.ErrNotFound) {
		return nil, ErrNotFound
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
	return t.collection.db.notifyTxnEvents(node, t.credentials)
}

// Discard discards all changes done in the current
// transaction.
func (t *Txn) Discard() {
	t.discarded = true
}

func getInstanceID(t []byte) core.InstanceID {
	partial := &struct{ ID *string }{}
	if err := json.Unmarshal(t, partial); err != nil {
		log.Fatalf("error when unmarshaling json instance: %v", err)
	}
	if partial.ID == nil {
		log.Fatal("invalid instance: doesn't have an ID attribute")
	}
	if *partial.ID != "" && !core.IsValidInstanceID(*partial.ID) {
		log.Fatal("invalid instance: invalid ID value")
	}
	return core.InstanceID(*partial.ID)
}

func setNewInstanceID(t []byte) (core.InstanceID, []byte) {
	newID := core.NewInstanceID()
	patchedValue, err := jsonpatch.MergePatch(t, []byte(fmt.Sprintf(`{"ID": %q}`, newID.String())))
	if err != nil {
		log.Fatalf("error while automatically patching autogenerated ID: %v", err)
	}
	return newID, patchedValue
}

func hasIDProperty(properties map[string]*jsonschema.Type) bool {
	idProperty := properties["ID"]
	if idProperty == nil || idProperty.Type != "string" {
		return false
	}
	return true
}
