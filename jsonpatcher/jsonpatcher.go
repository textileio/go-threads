package jsonpatcher

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	ds "github.com/ipfs/go-datastore"
	cbornode "github.com/ipfs/go-ipld-cbor"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	mh "github.com/multiformats/go-multihash"
	core "github.com/textileio/go-textile-core/store"
)

type operationType int

const (
	create operationType = iota
	save
	delete
)

var (
	log                           = logging.Logger("jsonpatcher")
	errSavingNonExistentInstance  = errors.New("can't save nonexistent instance")
	errCantCreateExistingInstance = errors.New("cant't create already existent instance")
	errUnknownOperation           = errors.New("unknown operation type")
)

type operation struct {
	Type      operationType
	EntityID  core.EntityID
	JSONPatch []byte
}

type jsonPatcher struct {
	jsonMode bool
}

var _ core.EventCodec = (*jsonPatcher)(nil)

func init() {
	cbornode.RegisterCborType(patchEvent{})
	cbornode.RegisterCborType(time.Time{})
}

// New returns a JSON-Patcher EventCodec
func New(jsonMode bool) core.EventCodec {
	return &jsonPatcher{jsonMode: jsonMode}
}

func (jp *jsonPatcher) Create(actions []core.Action) ([]core.Event, error) {
	events := make([]core.Event, len(actions))
	for i := range actions {
		var eventPayload []byte
		var err error
		switch actions[i].Type {
		case core.Create:
			eventPayload, err = createEvent(actions[i].EntityID, actions[i].Current, jp.jsonMode)
		case core.Save:
			eventPayload, err = saveEvent(actions[i].EntityID, actions[i].Previous, actions[i].Current, jp.jsonMode)
		case core.Delete:
			eventPayload, err = deleteEvent(actions[i].EntityID)
		default:
			panic("unkown action type")
		}
		if err != nil {
			return nil, err
		}
		events[i] = patchEvent{
			Timestamp: time.Now(),
			ID:        actions[i].EntityID,
			TypeName:  actions[i].EntityType,
			Patch:     eventPayload,
		}
	}
	return events, nil
}

func (jp *jsonPatcher) Reduce(e core.Event, datastore ds.Datastore, baseKey ds.Key) ([]core.ReduceAction, error) {
	var op operation
	if err := json.Unmarshal(e.Body(), &op); err != nil {
		return nil, err
	}

	var action core.ReduceAction
	key := baseKey.ChildString(e.EntityID().String())
	switch op.Type {
	case create:
		exist, err := datastore.Has(key)
		if err != nil {
			return nil, err
		}
		if exist {
			return nil, errCantCreateExistingInstance
		}
		if err := datastore.Put(key, op.JSONPatch); err != nil {
			return nil, fmt.Errorf("error when reducing create event: %v", err)
		}
		action = core.ReduceAction{Type: core.Create, EntityID: e.EntityID()}
		log.Debug("\tcreate operation applied")
	case save:
		value, err := datastore.Get(key)
		if errors.Is(err, ds.ErrNotFound) {
			return nil, errSavingNonExistentInstance
		}
		if err != nil {
			return nil, err
		}
		patchedValue, err := jsonpatch.MergePatch(value, op.JSONPatch)
		if err != nil {
			return nil, fmt.Errorf("error when reducing save event: %v", err)
		}
		if err = datastore.Put(key, patchedValue); err != nil {
			return nil, err
		}
		action = core.ReduceAction{Type: core.Save, EntityID: e.EntityID()}
		log.Debug("\tsave operation applied")
	case delete:
		if err := datastore.Delete(key); err != nil {
			return nil, err
		}
		action = core.ReduceAction{Type: core.Delete, EntityID: e.EntityID()}
		log.Debug("\tdelete operation applied")
	default:
		return nil, errUnknownOperation
	}

	return []core.ReduceAction{action}, nil
}

// EventFromBytes returns a unmarshaled event from its bytes representation
func (jp *jsonPatcher) EventFromBytes(data []byte) (core.Event, error) {
	event := &patchEvent{}
	if err := cbornode.DecodeInto(data, &event); err != nil {
		return nil, err
	}

	var op operation
	if err := json.Unmarshal(event.Patch, &op); err != nil {
		log.Errorf("error while unmarshaling patch: %v", err)
		return event, nil
	}
	log.Debug("unmarshaled event patch: %s", op.JSONPatch)
	return event, nil
}

func createEvent(id core.EntityID, v interface{}, jsonMode bool) ([]byte, error) {
	var opBytes []byte

	if jsonMode {
		strjson := v.(*string)
		opBytes = []byte(*strjson)
	} else {
		var err error
		opBytes, err = json.Marshal(v)
		if err != nil {
			return nil, err
		}
	}
	op := operation{
		Type:      create,
		EntityID:  id,
		JSONPatch: opBytes,
	}
	eventPayload, err := json.Marshal(op)
	if err != nil {
		return nil, err
	}
	return eventPayload, nil
}

func saveEvent(id core.EntityID, prev interface{}, curr interface{}, jsonMode bool) ([]byte, error) {
	var prevBytes, currBytes []byte
	if jsonMode {
		strCurrJson := curr.(*string)

		prevBytes = prev.([]byte)
		currBytes = []byte(*strCurrJson)
	} else {
		var err error
		prevBytes, err = json.Marshal(prev)
		if err != nil {
			return nil, err
		}
		currBytes, err = json.Marshal(curr)
		if err != nil {
			return nil, err
		}
	}
	jsonPatch, err := jsonpatch.CreateMergePatch(prevBytes, currBytes)
	if err != nil {
		return nil, err
	}
	op := operation{
		Type:      save,
		EntityID:  id,
		JSONPatch: jsonPatch,
	}
	eventPayload, err := json.Marshal(op)
	if err != nil {
		return nil, err
	}
	return eventPayload, nil
}

func deleteEvent(id core.EntityID) ([]byte, error) {
	op := operation{
		Type:      delete,
		EntityID:  id,
		JSONPatch: nil,
	}
	eventPayload, err := json.Marshal(op)
	if err != nil {
		return nil, err
	}
	return eventPayload, nil
}

type patchEvent struct {
	Timestamp time.Time
	ID        core.EntityID
	TypeName  string
	Patch     []byte
}

func (je patchEvent) Body() []byte {
	return je.Patch
}

func (je patchEvent) Time() []byte {
	t := je.Timestamp.UnixNano()
	buf := new(bytes.Buffer)
	// Use big endian to preserve lexicographic sorting
	_ = binary.Write(buf, binary.BigEndian, t)
	return buf.Bytes()
}

func (je patchEvent) EntityID() core.EntityID {
	return je.ID
}

func (je patchEvent) Type() string {
	return je.TypeName
}

func (je patchEvent) Node() (format.Node, error) {
	return cbornode.WrapObject(je, mh.SHA2_256, -1)
}

var _ core.Event = (*patchEvent)(nil)
