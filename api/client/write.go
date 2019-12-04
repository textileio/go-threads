package client

import (
	"encoding/json"
	"fmt"
	"reflect"

	pb "github.com/textileio/go-textile-threads/api/pb"
	es "github.com/textileio/go-textile-threads/eventstore"
)

// WriteTransaction encapsulates a write transaction
type WriteTransaction struct {
	client             pb.API_WriteTransactionClient
	storeID, modelName string
}

// Start starts the write transaction
func (t *WriteTransaction) Start() (EndTransactionFunc, error) {
	innerReq := &pb.StartTransactionRequest{StoreID: t.storeID, ModelName: t.modelName}
	option := &pb.WriteTransactionRequest_StartTransactionRequest{StartTransactionRequest: innerReq}
	if err := t.client.Send(&pb.WriteTransactionRequest{Option: option}); err != nil {
		return nil, err
	}
	return t.end, nil
}

// Has runs a has query in the active transaction
func (t *WriteTransaction) Has(entityIDs ...string) (bool, error) {
	innerReq := &pb.ModelHasRequest{EntityIDs: entityIDs}
	option := &pb.WriteTransactionRequest_ModelHasRequest{ModelHasRequest: innerReq}
	var err error
	if err = t.client.Send(&pb.WriteTransactionRequest{Option: option}); err != nil {
		return false, err
	}
	var resp *pb.WriteTransactionReply
	if resp, err = t.client.Recv(); err != nil {
		return false, err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_ModelHasReply:
		return x.ModelHasReply.GetExists(), nil
	default:
		return false, fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
	}
}

// FindByID gets the entity with the specified ID
func (t *WriteTransaction) FindByID(entityID string, entity interface{}) error {
	innerReq := &pb.ModelFindByIDRequest{EntityID: entityID}
	option := &pb.WriteTransactionRequest_ModelFindByIDRequest{ModelFindByIDRequest: innerReq}
	var err error
	if err = t.client.Send(&pb.WriteTransactionRequest{Option: option}); err != nil {
		return err
	}
	var resp *pb.WriteTransactionReply
	if resp, err = t.client.Recv(); err != nil {
		return err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_ModelFindByIDReply:
		err := json.Unmarshal([]byte(x.ModelFindByIDReply.GetEntity()), entity)
		return err
	default:
		return fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
	}
}

// Find finds entities by query
func (t *WriteTransaction) Find(query es.JSONQuery, dummySlice interface{}) (interface{}, error) {
	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	innerReq := &pb.ModelFindRequest{QueryJSON: queryBytes}
	option := &pb.WriteTransactionRequest_ModelFindRequest{ModelFindRequest: innerReq}
	if err = t.client.Send(&pb.WriteTransactionRequest{Option: option}); err != nil {
		return nil, err
	}
	var resp *pb.WriteTransactionReply
	if resp, err = t.client.Recv(); err != nil {
		return nil, err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_ModelFindReply:
		sliceType := reflect.TypeOf(dummySlice)
		elementType := sliceType.Elem().Elem()
		length := len(x.ModelFindReply.GetEntities())
		results := reflect.MakeSlice(sliceType, length, length)
		for i, result := range x.ModelFindReply.GetEntities() {
			target := reflect.New(elementType).Interface()
			err := json.Unmarshal(result, target)
			if err != nil {
				return nil, err
			}
			val := results.Index(i)
			val.Set(reflect.ValueOf(target))
		}
		return results.Interface(), nil
	default:
		return nil, fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
	}
}

// Create creates new instances of model objects
func (t *WriteTransaction) Create(items ...interface{}) error {
	values, err := marshalItems(items)
	if err != nil {
		return err
	}
	innerReq := &pb.ModelCreateRequest{
		Values: values,
	}
	option := &pb.WriteTransactionRequest_ModelCreateRequest{ModelCreateRequest: innerReq}
	if err = t.client.Send(&pb.WriteTransactionRequest{Option: option}); err != nil {
		return err
	}
	var resp *pb.WriteTransactionReply
	if resp, err = t.client.Recv(); err != nil {
		return err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_ModelCreateReply:
		for i, entity := range x.ModelCreateReply.GetEntities() {
			err := json.Unmarshal([]byte(entity), items[i])
			if err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
	}
}

// Save saves existing instances
func (t *WriteTransaction) Save(items ...interface{}) error {
	values, err := marshalItems(items)
	if err != nil {
		return err
	}
	innerReq := &pb.ModelSaveRequest{
		Values: values,
	}
	option := &pb.WriteTransactionRequest_ModelSaveRequest{ModelSaveRequest: innerReq}
	if err = t.client.Send(&pb.WriteTransactionRequest{Option: option}); err != nil {
		return err
	}
	var resp *pb.WriteTransactionReply
	if resp, err = t.client.Recv(); err != nil {
		return err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_ModelSaveReply:
		return nil
	default:
		return fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
	}
}

// Delete deletes data
func (t *WriteTransaction) Delete(entityIDs ...string) error {
	innerReq := &pb.ModelDeleteRequest{
		EntityIDs: entityIDs,
	}
	option := &pb.WriteTransactionRequest_ModelDeleteRequest{ModelDeleteRequest: innerReq}
	if err := t.client.Send(&pb.WriteTransactionRequest{Option: option}); err != nil {
		return err
	}
	var resp *pb.WriteTransactionReply
	var err error
	if resp, err = t.client.Recv(); err != nil {
		return err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_ModelDeleteReply:
		return nil
	default:
		return fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
	}
}

// end ends the active transaction
func (t *WriteTransaction) end() error {
	return t.client.CloseSend()
}
