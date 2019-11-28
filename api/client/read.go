package client

import (
	"encoding/json"
	"fmt"

	pb "github.com/textileio/go-textile-threads/api/pb"
	es "github.com/textileio/go-textile-threads/eventstore"
)

// ReadTransaction encapsulates a read transaction
type ReadTransaction struct {
	client             pb.API_ReadTransactionClient
	storeID, modelName string
}

// Start starts the read transaction
func (t *ReadTransaction) Start() error {
	innerReq := &pb.StartTransactionRequest{StoreID: t.storeID, ModelName: t.modelName}
	option := &pb.ReadTransactionRequest_StartTransactionRequest{StartTransactionRequest: innerReq}
	return t.client.Send(&pb.ReadTransactionRequest{Option: option})
}

// Has runs a has query in the active transaction
func (t *ReadTransaction) Has(entityIDs ...string) (bool, error) {
	innerReq := &pb.ModelHasRequest{EntityIDs: entityIDs}
	option := &pb.ReadTransactionRequest_ModelHasRequest{ModelHasRequest: innerReq}
	var err error
	var resp *pb.ReadTransactionReply
	if err = t.client.Send(&pb.ReadTransactionRequest{Option: option}); err != nil {
		return false, err
	}
	if resp, err = t.client.Recv(); err != nil {
		return false, err
	}
	switch x := resp.GetOption().(type) {
	case *pb.ReadTransactionReply_ModelHasReply:
		return x.ModelHasReply.GetExists(), nil
	default:
		return false, fmt.Errorf("ReadTransactionReply.Option has unexpected type %T", x)
	}
}

// FindByID gets the entity with the specified ID
func (t *ReadTransaction) FindByID(entityID string, entity interface{}) error {
	innerReq := &pb.ModelFindByIDRequest{EntityID: entityID}
	option := &pb.ReadTransactionRequest_ModelFindByIDRequest{ModelFindByIDRequest: innerReq}
	var err error
	var resp *pb.ReadTransactionReply
	if err = t.client.Send(&pb.ReadTransactionRequest{Option: option}); err != nil {
		return err
	}
	if resp, err = t.client.Recv(); err != nil {
		return err
	}
	switch x := resp.GetOption().(type) {
	case *pb.ReadTransactionReply_ModelFindByIDReply:
		err := json.Unmarshal([]byte(x.ModelFindByIDReply.GetEntity()), entity)
		return err
	default:
		return fmt.Errorf("ReadTransactionReply.Option has unexpected type %T", x)
	}
}

// Find finds entities by query
func (t *ReadTransaction) Find(query es.JSONQuery) ([][]byte, error) {
	queryBytes, err := json.Marshal(query)
	if err != nil {
		return [][]byte{}, err
	}
	innerReq := &pb.ModelFindRequest{QueryJSON: queryBytes}
	option := &pb.ReadTransactionRequest_ModelFindRequest{ModelFindRequest: innerReq}
	var resp *pb.ReadTransactionReply
	if err = t.client.Send(&pb.ReadTransactionRequest{Option: option}); err != nil {
		return [][]byte{}, err
	}
	if resp, err = t.client.Recv(); err != nil {
		return [][]byte{}, err
	}
	switch x := resp.GetOption().(type) {
	case *pb.ReadTransactionReply_ModelFindReply:
		return x.ModelFindReply.GetEntities(), nil
	default:
		return [][]byte{}, fmt.Errorf("ReadTransactionReply.Option has unexpected type %T", x)
	}
}

// End ends the active transaction
func (t *ReadTransaction) End() error {
	return t.client.CloseSend()
}
