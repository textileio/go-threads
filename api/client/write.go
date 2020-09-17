package client

import (
	"encoding/json"
	"fmt"
	"io"

	pb "github.com/textileio/go-threads/api/pb"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
)

// WriteTransaction encapsulates a write transaction.
type WriteTransaction struct {
	client         pb.API_WriteTransactionClient
	dbID           thread.ID
	collectionName string
}

// Start starts the write transaction.
func (t *WriteTransaction) Start() (EndTransactionFunc, error) {
	innerReq := &pb.StartTransactionRequest{
		DbID:           t.dbID.Bytes(),
		CollectionName: t.collectionName,
	}
	option := &pb.WriteTransactionRequest_StartTransactionRequest{
		StartTransactionRequest: innerReq,
	}
	if err := t.client.Send(&pb.WriteTransactionRequest{
		Option: option,
	}); err != nil {
		return nil, err
	}
	return t.end, nil
}

// Has runs a has query in the active transaction.
func (t *WriteTransaction) Has(instanceIDs ...string) (bool, error) {
	innerReq := &pb.HasRequest{
		InstanceIDs: instanceIDs,
	}
	option := &pb.WriteTransactionRequest_HasRequest{
		HasRequest: innerReq,
	}
	if err := t.client.Send(&pb.WriteTransactionRequest{
		Option: option,
	}); err != nil {
		return false, err
	}
	var resp *pb.WriteTransactionReply
	var err error
	if resp, err = t.client.Recv(); err != nil {
		return false, err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_HasReply:
		return x.HasReply.GetExists(), txnError(x.HasReply.TransactionError)
	default:
		return false, fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
	}
}

// FindByID gets the instance with the specified ID.
func (t *WriteTransaction) FindByID(instanceID string, instance interface{}) error {
	innerReq := &pb.FindByIDRequest{
		InstanceID: instanceID,
	}
	option := &pb.WriteTransactionRequest_FindByIDRequest{
		FindByIDRequest: innerReq,
	}
	if err := t.client.Send(&pb.WriteTransactionRequest{
		Option: option,
	}); err != nil {
		return err
	}
	var resp *pb.WriteTransactionReply
	var err error
	if resp, err = t.client.Recv(); err != nil {
		return err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_FindByIDReply:
		err := txnError(x.FindByIDReply.TransactionError)
		if err != nil {
			return err
		}
		err = json.Unmarshal(x.FindByIDReply.GetInstance(), instance)
		return err
	default:
		return fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
	}
}

// Find finds instances by query.
func (t *WriteTransaction) Find(query *db.Query, dummy interface{}) (interface{}, error) {
	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	innerReq := &pb.FindRequest{
		QueryJSON: queryBytes,
	}
	option := &pb.WriteTransactionRequest_FindRequest{
		FindRequest: innerReq,
	}
	if err = t.client.Send(&pb.WriteTransactionRequest{
		Option: option,
	}); err != nil {
		return nil, err
	}
	var resp *pb.WriteTransactionReply
	if resp, err = t.client.Recv(); err != nil {
		return nil, err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_FindReply:
		return processFindReply(x.FindReply, dummy)
	default:
		return nil, fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
	}
}

// Create creates new instances of objects.
func (t *WriteTransaction) Create(items ...interface{}) ([]string, error) {
	values, err := marshalItems(items)
	if err != nil {
		return nil, err
	}
	innerReq := &pb.CreateRequest{
		Instances: values,
	}
	option := &pb.WriteTransactionRequest_CreateRequest{
		CreateRequest: innerReq,
	}
	if err = t.client.Send(&pb.WriteTransactionRequest{
		Option: option,
	}); err != nil {
		return nil, err
	}
	var resp *pb.WriteTransactionReply
	if resp, err = t.client.Recv(); err != nil {
		return nil, err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_CreateReply:
		return x.CreateReply.GetInstanceIDs(), txnError(x.CreateReply.TransactionError)
	default:
		return nil, fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
	}
}

// Verify verifies existing instance changes.
func (t *WriteTransaction) Verify(items ...interface{}) error {
	values, err := marshalItems(items)
	if err != nil {
		return err
	}
	innerReq := &pb.VerifyRequest{
		Instances: values,
	}
	option := &pb.WriteTransactionRequest_VerifyRequest{
		VerifyRequest: innerReq,
	}
	if err = t.client.Send(&pb.WriteTransactionRequest{
		Option: option,
	}); err != nil {
		return err
	}
	var resp *pb.WriteTransactionReply
	if resp, err = t.client.Recv(); err != nil {
		return err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_VerifyReply:
		return txnError(x.VerifyReply.TransactionError)
	default:
		return fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
	}
}

// Save saves existing instances.
func (t *WriteTransaction) Save(items ...interface{}) error {
	values, err := marshalItems(items)
	if err != nil {
		return err
	}
	innerReq := &pb.SaveRequest{
		Instances: values,
	}
	option := &pb.WriteTransactionRequest_SaveRequest{
		SaveRequest: innerReq,
	}
	if err = t.client.Send(&pb.WriteTransactionRequest{
		Option: option,
	}); err != nil {
		return err
	}
	var resp *pb.WriteTransactionReply
	if resp, err = t.client.Recv(); err != nil {
		return err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_SaveReply:
		return txnError(x.SaveReply.TransactionError)
	default:
		return fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
	}
}

// Delete deletes data.
func (t *WriteTransaction) Delete(instanceIDs ...string) error {
	innerReq := &pb.DeleteRequest{
		InstanceIDs: instanceIDs,
	}
	option := &pb.WriteTransactionRequest_DeleteRequest{
		DeleteRequest: innerReq,
	}
	if err := t.client.Send(&pb.WriteTransactionRequest{
		Option: option,
	}); err != nil {
		return err
	}
	var resp *pb.WriteTransactionReply
	var err error
	if resp, err = t.client.Recv(); err != nil {
		return err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_DeleteReply:
		return txnError(x.DeleteReply.TransactionError)
	default:
		return fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
	}
}

// Discard discards transaction changes.
func (t *WriteTransaction) Discard() error {
	option := &pb.WriteTransactionRequest_DiscardRequest{
		DiscardRequest: &pb.DiscardRequest{},
	}
	if err := t.client.Send(&pb.WriteTransactionRequest{
		Option: option,
	}); err != nil {
		return err
	}
	var resp *pb.WriteTransactionReply
	var err error
	if resp, err = t.client.Recv(); err != nil {
		return err
	}
	switch x := resp.GetOption().(type) {
	case *pb.WriteTransactionReply_DiscardReply:
		return nil
	default:
		return fmt.Errorf("WriteTransactionReply.Option has unexpected type %T", x)
	}
}

// end ends the active transaction.
func (t *WriteTransaction) end() error {
	if err := t.client.CloseSend(); err != nil {
		return err
	}
	if _, err := t.client.Recv(); err != nil && err != io.EOF {
		return err
	}
	return nil
}
