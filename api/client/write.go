package client

import (
	"encoding/json"
	"fmt"

	pb "github.com/textileio/go-textile-threads/api/pb"
	es "github.com/textileio/go-textile-threads/eventstore"
)

// WriteTransaction encapsulates a read transaction
type WriteTransaction struct {
	client             pb.API_WriteTransactionClient
	storeID, modelName string
}

// Start starts the read transaction
func (t *WriteTransaction) Start() error {
	innerReq := &pb.StartTransactionRequest{StoreID: t.storeID, ModelName: t.modelName}
	option := &pb.WriteTransactionRequest_StartTransactionRequest{StartTransactionRequest: innerReq}
	return t.client.Send(&pb.WriteTransactionRequest{Option: option})
}

// Has runs a has query in the active transaction
func (t *WriteTransaction) Has(entityIDs ...string) (bool, error) {
	innerReq := &pb.ModelHasRequest{EntityIDs: entityIDs}
	option := &pb.WriteTransactionRequest_ModelHasRequest{ModelHasRequest: innerReq}
	var err error
	var resp *pb.WriteTransactionReply
	if err = t.client.Send(&pb.WriteTransactionRequest{Option: option}); err != nil {
		return false, err
	}
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
	var resp *pb.WriteTransactionReply
	if err = t.client.Send(&pb.WriteTransactionRequest{Option: option}); err != nil {
		return err
	}
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
func (t *WriteTransaction) Find(query es.JSONQuery) ([][]byte, error) {
	queryBytes, err := json.Marshal(query)
	if err != nil {
		return [][]byte{}, err
	}
	innerReq := &pb.ModelFindRequest{QueryJSON: queryBytes}
	option := &pb.WriteTransactionRequest_ModelFindRequest{ModelFindRequest: innerReq}
	var resp *pb.WriteTransactionReply
	if err = t.client.Send(&pb.WriteTransactionRequest{Option: option}); err != nil {
		return [][]byte{}, err
	}
	if resp, err = t.client.Recv(); err != nil {
		return [][]byte{}, err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_ModelFindReply:
		return x.ModelFindReply.GetEntities(), nil
	default:
		return [][]byte{}, fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
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
	var resp *pb.WriteTransactionReply
	if err = t.client.Send(&pb.WriteTransactionRequest{Option: option}); err != nil {
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
	var resp *pb.WriteTransactionReply
	if err = t.client.Send(&pb.WriteTransactionRequest{Option: option}); err != nil {
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
	var resp *pb.WriteTransactionReply
	if err := t.client.Send(&pb.WriteTransactionRequest{Option: option}); err != nil {
		return err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_ModelDeleteReply:
		return nil
	default:
		return fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
	}
}

// End ends the active transaction
func (t *WriteTransaction) End() error {
	return t.client.CloseSend()
}
