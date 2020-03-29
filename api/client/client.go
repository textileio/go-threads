package client

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	ic "github.com/libp2p/go-libp2p-core/crypto"
	ma "github.com/multiformats/go-multiaddr"
	pb "github.com/textileio/go-threads/api/pb"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ActionType describes the type of event action when subscribing to data updates
type ActionType int

const (
	// ActionCreate represents an event for creating a new instance
	ActionCreate ActionType = iota + 1
	// ActionSave represents an event for saving changes to an existing instance
	ActionSave
	// ActionDelete represents an event for deleting existing instance
	ActionDelete
)

// Action represents a data event delivered to a listener
type Action struct {
	Collection string
	Type       ActionType
	InstanceID string
	Instance   []byte
}

// ListenActionType describes the type of event action when receiving data updates
type ListenActionType int

const (
	// ListenAll specifies that Create, Save, and Delete events should be listened for
	ListenAll ListenActionType = iota
	// ListenCreate specifies that Create events should be listened for
	ListenCreate
	// ListenSave specifies that Save events should be listened for
	ListenSave
	// ListenDelete specifies that Delete events should be listened for
	ListenDelete
)

// ListenOption represents a filter to apply when listening for data updates
type ListenOption struct {
	Type       ListenActionType
	Collection string
	InstanceID string
}

// ListenEvent is used to send data or error values for Listen
type ListenEvent struct {
	Action Action
	Err    error
}

// Client provides the client api
type Client struct {
	c    pb.APIClient
	conn *grpc.ClientConn
}

// NewClient starts the client
func NewClient(target string, opts ...grpc.DialOption) (*Client, error) {
	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{
		c:    pb.NewAPIClient(conn),
		conn: conn,
	}, nil
}

// Close closes the client's grpc connection and cancels any active requests
func (c *Client) Close() error {
	return c.conn.Close()
}

// NewDB creates a new DB with ID
func (c *Client) NewDB(ctx context.Context, creds thread.Auth) error {
	signed, err := signCreds(creds)
	if err != nil {
		return err
	}
	_, err = c.c.NewDB(ctx, &pb.NewDBRequest{
		Credentials: signed,
	})
	return err
}

// NewDBFromAddr creates a new DB with address and keys.
func (c *Client) NewDBFromAddr(ctx context.Context, creds thread.Auth, addr ma.Multiaddr, key thread.Key, collections ...db.CollectionConfig) error {
	signed, err := signCreds(creds)
	if err != nil {
		return err
	}
	pbcollections := make([]*pb.CollectionConfig, len(collections))
	for i, c := range collections {
		cc, err := collectionConfigToPb(c)
		if err != nil {
			return err
		}
		pbcollections[i] = cc
	}
	_, err = c.c.NewDBFromAddr(ctx, &pb.NewDBFromAddrRequest{
		Credentials: signed,
		Addr:        addr.String(),
		Key:         key.Bytes(),
		Collections: pbcollections,
	})
	return err
}

func collectionConfigToPb(c db.CollectionConfig) (*pb.CollectionConfig, error) {
	idx := make([]*pb.CollectionConfig_IndexConfig, len(c.Indexes))
	for i, index := range c.Indexes {
		idx[i] = &pb.CollectionConfig_IndexConfig{
			Path:   index.Path,
			Unique: index.Unique,
		}
	}
	schemaBytes, err := json.Marshal(c.Schema)
	if err != nil {
		return nil, err
	}
	return &pb.CollectionConfig{
		Name:    c.Name,
		Schema:  schemaBytes,
		Indexes: idx,
	}, nil
}

// GetDBInfo retrives db addresses and keys.
func (c *Client) GetDBInfo(ctx context.Context, creds thread.Auth) (*pb.GetDBInfoReply, error) {
	signed, err := signCreds(creds)
	if err != nil {
		return nil, err
	}
	return c.c.GetDBInfo(ctx, &pb.GetDBInfoRequest{
		Credentials: signed,
	})
}

// NewCollection creates a new collection
func (c *Client) NewCollection(ctx context.Context, creds thread.Auth, config db.CollectionConfig) error {
	signed, err := signCreds(creds)
	if err != nil {
		return err
	}
	cc, err := collectionConfigToPb(config)
	if err != nil {
		return err
	}
	_, err = c.c.NewCollection(ctx, &pb.NewCollectionRequest{
		Credentials: signed,
		Config:      cc,
	})
	return err
}

// Create creates new instances of objects
func (c *Client) Create(ctx context.Context, creds thread.Auth, collectionName string, items ...interface{}) ([]string, error) {
	signed, err := signCreds(creds)
	if err != nil {
		return nil, err
	}
	instances, err := marshalItems(items)
	if err != nil {
		return nil, err
	}

	resp, err := c.c.Create(ctx, &pb.CreateRequest{
		Credentials:    signed,
		CollectionName: collectionName,
		Instances:      instances,
	})
	if err != nil {
		return nil, err
	}

	return resp.GetInstanceIDs(), nil
}

// Save saves existing instances
func (c *Client) Save(ctx context.Context, creds thread.Auth, collectionName string, instances ...interface{}) error {
	signed, err := signCreds(creds)
	if err != nil {
		return err
	}
	values, err := marshalItems(instances)
	if err != nil {
		return err
	}

	_, err = c.c.Save(ctx, &pb.SaveRequest{
		Credentials:    signed,
		CollectionName: collectionName,
		Instances:      values,
	})
	return err
}

// Delete deletes data
func (c *Client) Delete(ctx context.Context, creds thread.Auth, collectionName string, instanceIDs ...string) error {
	signed, err := signCreds(creds)
	if err != nil {
		return err
	}
	_, err = c.c.Delete(ctx, &pb.DeleteRequest{
		Credentials:    signed,
		CollectionName: collectionName,
		InstanceIDs:    instanceIDs,
	})
	return err
}

// Has checks if the specified instances exist
func (c *Client) Has(ctx context.Context, creds thread.Auth, collectionName string, instanceIDs ...string) (bool, error) {
	signed, err := signCreds(creds)
	if err != nil {
		return false, err
	}
	resp, err := c.c.Has(ctx, &pb.HasRequest{
		Credentials:    signed,
		CollectionName: collectionName,
		InstanceIDs:    instanceIDs,
	})
	if err != nil {
		return false, err
	}
	return resp.GetExists(), nil
}

// Find finds instances by query
func (c *Client) Find(ctx context.Context, creds thread.Auth, collectionName string, query *db.Query, dummySlice interface{}) (interface{}, error) {
	signed, err := signCreds(creds)
	if err != nil {
		return nil, err
	}
	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	resp, err := c.c.Find(ctx, &pb.FindRequest{
		Credentials:    signed,
		CollectionName: collectionName,
		QueryJSON:      queryBytes,
	})
	if err != nil {
		return nil, err
	}
	return processFindReply(resp, dummySlice)
}

// FindByID finds an instance by id
func (c *Client) FindByID(ctx context.Context, creds thread.Auth, collectionName, instanceID string, instance interface{}) error {
	signed, err := signCreds(creds)
	if err != nil {
		return err
	}
	resp, err := c.c.FindByID(ctx, &pb.FindByIDRequest{
		Credentials:    signed,
		CollectionName: collectionName,
		InstanceID:     instanceID,
	})
	if err != nil {
		return err
	}
	return json.Unmarshal(resp.GetInstance(), instance)
}

// ReadTransaction returns a read transaction that can be started and used and ended
func (c *Client) ReadTransaction(ctx context.Context, creds thread.Auth, collectionName string) (*ReadTransaction, error) {
	client, err := c.c.ReadTransaction(ctx)
	if err != nil {
		return nil, err
	}
	return &ReadTransaction{
		client:         client,
		creds:          creds,
		collectionName: collectionName,
	}, nil
}

// WriteTransaction returns a read transaction that can be started and used and ended
func (c *Client) WriteTransaction(ctx context.Context, creds thread.Auth, collectionName string) (*WriteTransaction, error) {
	client, err := c.c.WriteTransaction(ctx)
	if err != nil {
		return nil, err
	}
	return &WriteTransaction{
		client:         client,
		creds:          creds,
		collectionName: collectionName,
	}, nil
}

// Listen provides an update whenever the specified db, collection, or instance is updated
func (c *Client) Listen(ctx context.Context, creds thread.Auth, listenOptions ...ListenOption) (<-chan ListenEvent, error) {
	signed, err := signCreds(creds)
	if err != nil {
		return nil, err
	}
	channel := make(chan ListenEvent)
	filters := make([]*pb.ListenRequest_Filter, len(listenOptions))
	for i, listenOption := range listenOptions {
		var action pb.ListenRequest_Filter_Action
		switch listenOption.Type {
		case ListenAll:
			action = pb.ListenRequest_Filter_ALL
		case ListenCreate:
			action = pb.ListenRequest_Filter_CREATE
		case ListenDelete:
			action = pb.ListenRequest_Filter_DELETE
		case ListenSave:
			action = pb.ListenRequest_Filter_SAVE
		default:
			return nil, fmt.Errorf("unknown ListenOption.Type %v", listenOption.Type)
		}
		filters[i] = &pb.ListenRequest_Filter{
			CollectionName: listenOption.Collection,
			InstanceID:     listenOption.InstanceID,
			Action:         action,
		}
	}
	stream, err := c.c.Listen(ctx, &pb.ListenRequest{
		Credentials: signed,
		Filters:     filters,
	})
	if err != nil {
		return nil, err
	}
	go func() {
		defer close(channel)

	loop:
		for {
			event, err := stream.Recv()
			if err != nil {
				stat := status.Convert(err)
				if stat == nil || (stat.Code() != codes.Canceled) {
					channel <- ListenEvent{Err: err}
				}
				break
			}
			var actionType ActionType
			switch event.GetAction() {
			case pb.ListenReply_CREATE:
				actionType = ActionCreate
			case pb.ListenReply_DELETE:
				actionType = ActionDelete
			case pb.ListenReply_SAVE:
				actionType = ActionSave
			default:
				channel <- ListenEvent{Err: fmt.Errorf("unknown listen reply action %v", event.GetAction())}
				break loop
			}
			action := Action{
				Collection: event.GetCollectionName(),
				Type:       actionType,
				InstanceID: event.GetInstanceID(),
				Instance:   event.GetInstance(),
			}
			channel <- ListenEvent{Action: action}
		}
	}()
	return channel, nil
}

func processFindReply(reply *pb.FindReply, dummySlice interface{}) (interface{}, error) {
	sliceType := reflect.TypeOf(dummySlice)
	elementType := sliceType.Elem().Elem()
	length := len(reply.GetInstances())
	results := reflect.MakeSlice(sliceType, length, length)
	for i, result := range reply.GetInstances() {
		target := reflect.New(elementType).Interface()
		err := json.Unmarshal(result, target)
		if err != nil {
			return nil, err
		}
		val := results.Index(i)
		val.Set(reflect.ValueOf(target))
	}
	return results.Interface(), nil
}

func marshalItems(items []interface{}) ([][]byte, error) {
	values := make([][]byte, len(items))
	for i, item := range items {
		bytes, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}
		values[i] = bytes
	}
	return values, nil
}

func signCreds(creds thread.Auth) (pcreds *pb.Credentials, err error) {
	pcreds = &pb.Credentials{
		ThreadID: creds.ThreadID().Bytes(),
	}
	pk, sig, err := creds.Sign()
	if err != nil {
		return
	}
	if pk == nil {
		return
	}
	pcreds.PubKey, err = ic.MarshalPublicKey(pk)
	if err != nil {
		return
	}
	pcreds.Signature = sig
	return pcreds, nil
}
