package client

import (
	"encoding/json"
	"fmt"

	pb "github.com/textileio/go-threads/api/pb"
	"github.com/textileio/go-threads/db"
)

// ReadTransaction encapsulates a read transaction
type ReadTransaction struct {
	client          pb.API_ReadTransactionClient
	dbID, modelName string
}

// EndTransactionFunc must be called to end a transaction after it has been started
type EndTransactionFunc = func() error

// Start starts the read transaction
func (t *ReadTransaction) Start() (EndTransactionFunc, error) {
	innerReq := &pb.StartTransactionRequest{DBID: t.dbID, ModelName: t.modelName}
	option := &pb.ReadTransactionRequest_StartTransactionRequest{StartTransactionRequest: innerReq}
	if err := t.client.Send(&pb.ReadTransactionRequest{Option: option}); err != nil {
		return nil, err
	}
	return t.end, nil
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
func (t *ReadTransaction) Find(query *db.JSONQuery, dummySlice interface{}) (interface{}, error) {
	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	innerReq := &pb.ModelFindRequest{QueryJSON: queryBytes}
	option := &pb.ReadTransactionRequest_ModelFindRequest{ModelFindRequest: innerReq}
	var resp *pb.ReadTransactionReply
	if err = t.client.Send(&pb.ReadTransactionRequest{Option: option}); err != nil {
		return nil, err
	}
	if resp, err = t.client.Recv(); err != nil {
		return nil, err
	}
	switch x := resp.GetOption().(type) {
	case *pb.ReadTransactionReply_ModelFindReply:
		return processFindReply(x.ModelFindReply, dummySlice)
	default:
		return nil, fmt.Errorf("ReadTransactionReply.Option has unexpected type %T", x)
	}
}

// end ends the active transaction
func (t *ReadTransaction) end() error {
	return t.client.CloseSend()
}
