package client

import (
	"encoding/json"
	"fmt"
	"io"

	pb "github.com/textileio/go-threads/api/pb"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
)

// ReadTransaction encapsulates a read transaction.
type ReadTransaction struct {
	client         pb.API_ReadTransactionClient
	dbID           thread.ID
	collectionName string
}

// EndTransactionFunc must be called to end a transaction after it has been started.
type EndTransactionFunc = func() error

// Start starts the read transaction.
func (t *ReadTransaction) Start() (EndTransactionFunc, error) {
	innerReq := &pb.StartTransactionRequest{
		DbID:           t.dbID.Bytes(),
		CollectionName: t.collectionName,
	}
	option := &pb.ReadTransactionRequest_StartTransactionRequest{
		StartTransactionRequest: innerReq,
	}
	if err := t.client.Send(&pb.ReadTransactionRequest{
		Option: option,
	}); err != nil {
		return nil, err
	}
	return t.end, nil
}

// Has runs a has query in the active transaction.
func (t *ReadTransaction) Has(instanceIDs ...string) (bool, error) {
	innerReq := &pb.HasRequest{
		InstanceIDs: instanceIDs,
	}
	option := &pb.ReadTransactionRequest_HasRequest{
		HasRequest: innerReq,
	}
	if err := t.client.Send(&pb.ReadTransactionRequest{
		Option: option,
	}); err != nil {
		return false, err
	}
	var resp *pb.ReadTransactionReply
	var err error
	if resp, err = t.client.Recv(); err != nil {
		return false, err
	}
	switch x := resp.GetOption().(type) {
	case *pb.ReadTransactionReply_HasReply:
		return x.HasReply.GetExists(), txnError(x.HasReply.TransactionError)
	default:
		return false, fmt.Errorf("ReadTransactionReply.Option has unexpected type %T", x)
	}
}

// FindByID gets the instance with the specified ID.
func (t *ReadTransaction) FindByID(instanceID string, instance interface{}) error {
	innerReq := &pb.FindByIDRequest{
		InstanceID: instanceID,
	}
	option := &pb.ReadTransactionRequest_FindByIDRequest{
		FindByIDRequest: innerReq,
	}
	if err := t.client.Send(&pb.ReadTransactionRequest{
		Option: option,
	}); err != nil {
		return err
	}
	var resp *pb.ReadTransactionReply
	var err error
	if resp, err = t.client.Recv(); err != nil {
		return err
	}
	switch x := resp.GetOption().(type) {
	case *pb.ReadTransactionReply_FindByIDReply:
		err := txnError(x.FindByIDReply.TransactionError)
		if err != nil {
			return err
		}
		err = json.Unmarshal(x.FindByIDReply.GetInstance(), instance)
		return err
	default:
		return fmt.Errorf("ReadTransactionReply.Option has unexpected type %T", x)
	}
}

// Find finds instances by query.
func (t *ReadTransaction) Find(query *db.Query, dummy interface{}) (interface{}, error) {
	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	innerReq := &pb.FindRequest{
		QueryJSON: queryBytes,
	}
	option := &pb.ReadTransactionRequest_FindRequest{
		FindRequest: innerReq,
	}
	if err := t.client.Send(&pb.ReadTransactionRequest{
		Option: option,
	}); err != nil {
		return nil, err
	}
	var resp *pb.ReadTransactionReply
	if resp, err = t.client.Recv(); err != nil {
		return nil, err
	}
	switch x := resp.GetOption().(type) {
	case *pb.ReadTransactionReply_FindReply:
		return processFindReply(x.FindReply, dummy)
	default:
		return nil, fmt.Errorf("ReadTransactionReply.Option has unexpected type %T", x)
	}
}

// end ends the active transaction.
func (t *ReadTransaction) end() error {
	if err := t.client.CloseSend(); err != nil {
		return err
	}
	if _, err := t.client.Recv(); err != nil && err != io.EOF {
		return err
	}
	return nil
}
