package client

import (
	"encoding/json"
	"fmt"

	pb "github.com/textileio/go-threads/api/pb"
	"github.com/textileio/go-threads/db"
)

// WriteTransaction encapsulates a write transaction
type WriteTransaction struct {
	client               pb.API_WriteTransactionClient
	dbID, collectionName string
}

// Start starts the write transaction
func (t *WriteTransaction) Start() (EndTransactionFunc, error) {
	innerReq := &pb.StartTransactionRequest{DBID: t.dbID, CollectionName: t.collectionName}
	option := &pb.WriteTransactionRequest_StartTransactionRequest{StartTransactionRequest: innerReq}
	if err := t.client.Send(&pb.WriteTransactionRequest{Option: option}); err != nil {
		return nil, err
	}
	return t.end, nil
}

// Has runs a has query in the active transaction
func (t *WriteTransaction) Has(entityIDs ...string) (bool, error) {
	innerReq := &pb.HasRequest{EntityIDs: entityIDs}
	option := &pb.WriteTransactionRequest_HasRequest{HasRequest: innerReq}
	var err error
	if err = t.client.Send(&pb.WriteTransactionRequest{Option: option}); err != nil {
		return false, err
	}
	var resp *pb.WriteTransactionReply
	if resp, err = t.client.Recv(); err != nil {
		return false, err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_HasReply:
		return x.HasReply.GetExists(), nil
	default:
		return false, fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
	}
}

// FindByID gets the entity with the specified ID
func (t *WriteTransaction) FindByID(entityID string, entity interface{}) error {
	innerReq := &pb.FindByIDRequest{EntityID: entityID}
	option := &pb.WriteTransactionRequest_FindByIDRequest{FindByIDRequest: innerReq}
	var err error
	if err = t.client.Send(&pb.WriteTransactionRequest{Option: option}); err != nil {
		return err
	}
	var resp *pb.WriteTransactionReply
	if resp, err = t.client.Recv(); err != nil {
		return err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_FindByIDReply:
		err := json.Unmarshal([]byte(x.FindByIDReply.GetEntity()), entity)
		return err
	default:
		return fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
	}
}

// Find finds entities by query
func (t *WriteTransaction) Find(query *db.JSONQuery, dummySlice interface{}) (interface{}, error) {
	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	innerReq := &pb.FindRequest{QueryJSON: queryBytes}
	option := &pb.WriteTransactionRequest_FindRequest{FindRequest: innerReq}
	if err = t.client.Send(&pb.WriteTransactionRequest{Option: option}); err != nil {
		return nil, err
	}
	var resp *pb.WriteTransactionReply
	if resp, err = t.client.Recv(); err != nil {
		return nil, err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_FindReply:
		return processFindReply(x.FindReply, dummySlice)
	default:
		return nil, fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
	}
}

// Create creates new instances of objects
func (t *WriteTransaction) Create(items ...interface{}) error {
	values, err := marshalItems(items)
	if err != nil {
		return err
	}
	innerReq := &pb.CreateRequest{
		Values: values,
	}
	option := &pb.WriteTransactionRequest_CreateRequest{CreateRequest: innerReq}
	if err = t.client.Send(&pb.WriteTransactionRequest{Option: option}); err != nil {
		return err
	}
	var resp *pb.WriteTransactionReply
	if resp, err = t.client.Recv(); err != nil {
		return err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_CreateReply:
		for i, entity := range x.CreateReply.GetEntities() {
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
	innerReq := &pb.SaveRequest{
		Values: values,
	}
	option := &pb.WriteTransactionRequest_SaveRequest{SaveRequest: innerReq}
	if err = t.client.Send(&pb.WriteTransactionRequest{Option: option}); err != nil {
		return err
	}
	var resp *pb.WriteTransactionReply
	if resp, err = t.client.Recv(); err != nil {
		return err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_SaveReply:
		return nil
	default:
		return fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
	}
}

// Delete deletes data
func (t *WriteTransaction) Delete(entityIDs ...string) error {
	innerReq := &pb.DeleteRequest{
		EntityIDs: entityIDs,
	}
	option := &pb.WriteTransactionRequest_DeleteRequest{DeleteRequest: innerReq}
	if err := t.client.Send(&pb.WriteTransactionRequest{Option: option}); err != nil {
		return err
	}
	var resp *pb.WriteTransactionReply
	var err error
	if resp, err = t.client.Recv(); err != nil {
		return err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_DeleteReply:
		return nil
	default:
		return fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
	}
}

// end ends the active transaction
func (t *WriteTransaction) end() error {
	return t.client.CloseSend()
}
