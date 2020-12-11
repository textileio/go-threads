package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/jsonschema"
	"github.com/dop251/goja"
	jsonpatch "github.com/evanphx/json-patch"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	format "github.com/ipfs/go-ipld-format"
	"github.com/textileio/go-threads/core/app"
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
	errAlreadyDiscardedCommitedTxn = errors.New("can't commit discarded/committed txn")
	errCantCreateExistingInstance  = errors.New("can't create already existing instance")

	baseKey = dsPrefix.ChildString("collection")
)

const (
	writeValidatorFn = "_validate"
	readFilterFn     = "_filter"
)

// Collection is a group of instances sharing a schema.
// Collections are like RDBMS tables. They can only exist in a single database.
type Collection struct {
	name              string
	schemaLoader      gojsonschema.JSONLoader
	db                *DB
	indexes           map[string]Index
	vm                *goja.Runtime
	rawWriteValidator []byte
	writeValidator    goja.Callable
	rawReadFilter     []byte
	readFilter        goja.Callable
	sync.Mutex
}

// newCollection returns a new Collection from schema.
func newCollection(d *DB, config CollectionConfig) (*Collection, error) {
	if config.Name != "" && !nameRx.MatchString(config.Name) {
		return nil, ErrInvalidName
	}
	idType, err := getSchemaTypeAtPath(config.Schema, idFieldName)
	if err != nil {
		if errors.Is(err, ErrInvalidCollectionSchemaPath) {
			return nil, ErrInvalidCollectionSchema
		}
		return nil, err
	}
	if idType.Type != "string" {
		return nil, ErrInvalidCollectionSchema
	}
	sb, err := json.Marshal(config.Schema)
	if err != nil {
		return nil, err
	}
	vm := goja.New()
	wv := []byte(config.WriteValidator)
	rf := []byte(config.ReadFilter)
	c := &Collection{
		name:              config.Name,
		schemaLoader:      gojsonschema.NewBytesLoader(sb),
		db:                d,
		indexes:           make(map[string]Index),
		vm:                vm,
		rawWriteValidator: wv,
		rawReadFilter:     rf,
	}
	wvObj, err := compileJSFunc(wv, writeValidatorFn, "writer", "event", "instance")
	if err != nil {
		return nil, err
	}
	if wvObj != nil {
		c.writeValidator, err = loadJSFunc(vm, writeValidatorFn, wvObj)
		if err != nil {
			return nil, err
		}
	}
	rfObj, err := compileJSFunc(rf, readFilterFn, "reader", "instance")
	if err != nil {
		return nil, err
	}
	if rfObj != nil {
		c.readFilter, err = loadJSFunc(vm, readFilterFn, rfObj)
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

// baseKey returns the collections base key.
func (c *Collection) baseKey() ds.Key {
	return baseKey.ChildString(c.name)
}

// GetName returns the collection name.
func (c *Collection) GetName() string {
	return c.name
}

// GetSchema returns the current collection schema.
func (c *Collection) GetSchema() []byte {
	return c.schemaLoader.JsonSource().([]byte)
}

// GetWriteValidator returns the current collection write validator.
func (c *Collection) GetWriteValidator() []byte {
	return c.rawWriteValidator
}

// GetReadFilter returns the current collection read filter.
func (c *Collection) GetReadFilter() []byte {
	return c.rawReadFilter
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
	err = c.ReadTxn(func(txn *Txn) error {
		instance, err = txn.FindByID(id)
		return err
	}, opts...)
	return
}

// Create creates an instance in the collection.
func (c *Collection) Create(v []byte, opts ...TxnOption) (id core.InstanceID, err error) {
	err = c.WriteTxn(func(txn *Txn) error {
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
	err = c.WriteTxn(func(txn *Txn) error {
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

// DeleteMany deletes multiple instances by ID. It doesn't
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

// Verify verifies changes of an instance in the collection.
func (c *Collection) Verify(v []byte, opts ...TxnOption) error {
	return c.WriteTxn(func(txn *Txn) error {
		return txn.Verify(v)
	}, opts...)
}

// VerifyMany verifies changes of multiple instances in the collection.
func (c *Collection) VerifyMany(vs [][]byte, opts ...TxnOption) error {
	return c.WriteTxn(func(txn *Txn) error {
		return txn.Verify(vs...)
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

type filter struct {
	Collection string
	Time       int
}

func (f filter) Filter(e query.Entry) bool {
	// Easy out, are we in the right collection?
	key := ds.NewKey(e.Key)
	base := key.Parent().BaseNamespace()
	number, _ := strconv.Atoi(base)
	kind := key.Type()
	return kind == f.Collection && number > f.Time

}

// ModifiedSince returns a list of all instances that have been modified (and/or touched) since `time`.
func (c *Collection) ModifiedSince(time int64, opts ...TxnOption) (ids []core.InstanceID, err error) {
	_ = c.ReadTxn(func(txn *Txn) error {
		ids, err = txn.ModifiedSince(time)
		return err
	}, opts...)
	return
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

// validWrite validates new events against the identity and user-defined write validator function.
func (c *Collection) validWrite(identity thread.PubKey, e core.Event) error {
	c.Lock()
	defer c.Unlock()
	if c.writeValidator == nil {
		return nil
	}
	writer, err := loadJSIdentity(c.vm, identity)
	if err != nil {
		return err
	}
	data, err := e.Marshal()
	if err != nil {
		return fmt.Errorf("marshal event in validate write: %v", err)
	}
	event, err := parseJSON(c.vm, data)
	if err != nil {
		return fmt.Errorf("parsing event in validate write: %v", err)
	}
	var inv goja.Value
	val, err := c.db.datastore.Get(baseKey.ChildString(c.name).ChildString(e.InstanceID().String()))
	if err != nil && !errors.Is(err, ds.ErrNotFound) {
		return err
	}
	if val != nil {
		inv, err = parseJSON(c.vm, val)
		if err != nil {
			return fmt.Errorf("parsing instance in validate write: %v", err)
		}
	}
	res, err := c.writeValidator(nil, writer, event, inv)
	if err != nil {
		return fmt.Errorf("running write validator func: %v", err)
	}
	out := res.Export()
	switch out.(type) {
	case bool:
		if out.(bool) {
			return nil
		}
		return app.ErrInvalidNetRecordBody
	case nil:
		return app.ErrInvalidNetRecordBody
	default:
		return fmt.Errorf("%w: %v", app.ErrInvalidNetRecordBody, out)
	}
}

// filterRead filters an instance against the identity and user-defined read filter function.
func (c *Collection) filterRead(identity thread.PubKey, instance []byte) ([]byte, error) {
	c.Lock()
	defer c.Unlock()
	if c.readFilter == nil {
		return instance, nil
	}
	reader, err := loadJSIdentity(c.vm, identity)
	if err != nil {
		return nil, err
	}
	inv, err := parseJSON(c.vm, instance)
	if err != nil {
		return nil, fmt.Errorf("parsing instance in filter read: %v", err)
	}
	res, err := c.readFilter(nil, reader, inv)
	if err != nil {
		return nil, fmt.Errorf("running read filter func: %v", err)
	}
	out := res.Export()
	switch out.(type) {
	case nil:
		return nil, nil
	default:
		return json.Marshal(out)
	}
}

// Txn represents a read/write transaction in the db. It allows for
// serializable isolation level within the db.
type Txn struct {
	collection *Collection
	token      thread.Token
	discarded  bool
	committed  bool
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

		// Update readonly/protected mod tag
		_, updated = setModifiedTag(updated)

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

// Verify verifies updated instances but does not save them.
func (t *Txn) Verify(updated ...[]byte) error {
	identity, err := t.token.PubKey()
	if err != nil {
		return err
	}
	actions, err := t.createSaveActions(identity, updated...)
	if err != nil {
		return err
	}

	events, _, err := t.createEvents(actions)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}
	for _, e := range events {
		if err := t.collection.validWrite(identity, e); err != nil {
			return err
		}
	}
	return nil
}

// Save saves an instance changes to be committed when the current transaction commits.
func (t *Txn) Save(updated ...[]byte) error {
	identity, err := t.token.PubKey()
	if err != nil {
		return err
	}
	actions, err := t.createSaveActions(identity, updated...)
	if err != nil {
		return err
	}

	t.actions = append(t.actions, actions...)
	return nil
}

func (t *Txn) createSaveActions(identity thread.PubKey, updated ...[]byte) ([]core.Action, error) {
	var actions []core.Action
	for i := range updated {
		if t.readonly {
			return nil, ErrReadonlyTx
		}

		next := make([]byte, len(updated[i]))
		copy(next, updated[i])

		if err := t.collection.validInstance(next); err != nil {
			return nil, err
		}

		// Update readonly/protected mod tag
		_, next = setModifiedTag(next)

		// Because this is a save event, even though we might still create the new instance
		// it has to have a valid _id ahead of time.
		id, err := getInstanceID(next)
		if err != nil {
			return nil, err
		}
		key := baseKey.ChildString(t.collection.name).ChildString(id.String())
		previous, err := t.collection.db.datastore.Get(key)
		if err == ds.ErrNotFound {
			// Default to an empty doc, downstream reducer will take care of patching, etc
			previous = []byte("{}")
		} else if err != nil {
			return nil, err
		} else {
			// No errors, carry on
			previous, err = t.collection.filterRead(identity, previous)
			if err != nil {
				return nil, err
			}
		}

		actions = append(actions, core.Action{
			Type:           core.Save,
			InstanceID:     id,
			CollectionName: t.collection.name,
			Previous:       previous,
			Current:        next,
		})
	}
	return actions, nil
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
			// Nothing to be done here
			return nil
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
	if err := t.collection.db.connector.Validate(t.token, true); err != nil {
		return false, err
	}
	pk, err := t.token.PubKey()
	if err != nil {
		return false, err
	}
	for i := range ids {
		key := baseKey.ChildString(t.collection.name).ChildString(ids[i].String())
		exists, err := t.collection.db.datastore.Has(key)
		if err != nil {
			return false, err
		}
		if exists {
			if t.collection.readFilter == nil {
				continue
			}
			bytes, err := t.collection.db.datastore.Get(key)
			if err != nil {
				return false, err
			}
			bytes, err = t.collection.filterRead(pk, bytes)
			if err != nil {
				return false, err
			}
			if bytes == nil { // Access denied
				return false, nil
			}
		} else {
			return false, nil
		}
	}
	return true, nil
}

// FindByID gets an instance by ID in the current txn scope.
func (t *Txn) FindByID(id core.InstanceID) ([]byte, error) {
	if err := t.collection.db.connector.Validate(t.token, true); err != nil {
		return nil, err
	}
	key := baseKey.ChildString(t.collection.name).ChildString(id.String())
	bytes, err := t.collection.db.datastore.Get(key)
	if errors.Is(err, ds.ErrNotFound) {
		return nil, ErrInstanceNotFound
	}
	if err != nil {
		return nil, err
	}
	pk, err := t.token.PubKey()
	if err != nil {
		return nil, err
	}
	bytes, err = t.collection.filterRead(pk, bytes)
	if err != nil {
		return nil, err
	}
	if bytes == nil {
		return nil, ErrInstanceNotFound
	}
	return bytes, err
}

// Commit applies all changes done in the current transaction
// to the collection. This is a syncrhonous call so changes can
// be assumed to be applied on function return.
func (t *Txn) Commit() error {
	events, node, err := t.createEvents(t.actions)
	if err != nil {
		return err
	}
	if node == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), createNetRecordTimeout)
	defer cancel()
	if _, err = t.collection.db.connector.CreateNetRecord(ctx, node, t.token); err != nil {
		return err
	}
	if err = t.collection.db.dispatcher.Dispatch(events); err != nil {
		return err
	}
	return t.collection.db.notifyTxnEvents(node, t.token)
}

// Discard discards all changes done in the current transaction.
func (t *Txn) Discard() {
	t.discarded = true
}

// RefreshCollection updates the transaction's collection reference from the master db map,
// which may have received updates while the transaction is open.
func (t *Txn) RefreshCollection() error {
	c, ok := t.collection.db.collections[t.collection.name]
	if !ok {
		return ErrCollectionNotFound
	}
	t.collection = c
	return nil
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

// getSchemaTypeProperties extracts a map of schema properties from a given input schema.
// If there are no available properties, it will return an empty map
func getSchemaTypeProperties(jt *jsonschema.Type, defs jsonschema.Definitions) (map[string]*jsonschema.Type, error) {
	if jt == nil {
		return make(map[string]*jsonschema.Type), nil
	}
	properties := jt.Properties
	if jt.Ref != "" {
		parts := strings.Split(jt.Ref, "/")
		if len(parts) < 1 {
			return make(map[string]*jsonschema.Type), nil
		}
		def := defs[parts[len(parts)-1]]
		if def == nil {
			return make(map[string]*jsonschema.Type), nil
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
		return core.EmptyInstanceID, fmt.Errorf("unmarshaling json instance: %v", err)
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
		log.Fatalf("while automatically patching autogenerated _id: %v", err)
	}
	return newID, patchedValue
}

func setModifiedTag(t []byte) (newTime int64, patchedValue []byte) {
	newTime = time.Now().UnixNano()
	patchedValue, err := jsonpatch.MergePatch(t, []byte(fmt.Sprintf(`{"%s": %d}`, modFieldName, newTime)))
	if err != nil {
		log.Fatalf("while automatically patching autogenerated _mod: %v", err)
	}
	return
}

func (t *Txn) createEvents(actions []core.Action) (events []core.Event, node format.Node, err error) {
	if t.discarded || t.committed {
		return nil, nil, errAlreadyDiscardedCommitedTxn
	}
	events, node, err = t.collection.db.eventcodec.Create(actions)
	if err != nil {
		return
	}
	if len(events) == 0 && node == nil {
		return nil, nil, nil
	}
	if len(events) == 0 || node == nil {
		return nil, nil, fmt.Errorf("created events and node must both be nil or not-nil")
	}
	return events, node, nil
}

func compileJSFunc(v []byte, name string, args ...string) ([]byte, error) {
	if len(v) == 0 {
		return nil, nil
	}
	script := fmt.Sprintf(`function %s(%s) {%s}`, name, strings.Join(args, ","), string(v))
	script = strings.Replace(script, "\t", "", -1)
	if _, err := goja.Compile("", script, true); err != nil {
		return nil, fmt.Errorf("compiling js func: %v", err)
	}
	return []byte(script), nil
}

func loadJSFunc(vm *goja.Runtime, name string, obj []byte) (goja.Callable, error) {
	_, err := vm.RunString(string(obj))
	if err != nil {
		return nil, err
	}
	fn, ok := goja.AssertFunction(vm.Get(name))
	if !ok {
		return nil, fmt.Errorf("object is not a function: %s", string(obj))
	}
	return fn, nil
}

func loadJSIdentity(vm *goja.Runtime, identity thread.PubKey) (goja.Value, error) {
	if identity == nil {
		return nil, nil
	}
	return vm.RunString(fmt.Sprintf("'%s'", identity.String()))
}
