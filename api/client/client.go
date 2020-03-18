package client

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

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
func (c *Client) NewDB(ctx context.Context, id thread.ID) error {
	_, err := c.c.NewDB(ctx, &pb.NewDBRequest{
		DbID: id.String(),
	})
	return err
}

// NewDBFromAddr creates a new DB with address and keys.
func (c *Client) NewDBFromAddr(ctx context.Context, addr ma.Multiaddr, key thread.Key, collections ...db.CollectionConfig) error {
	pbcollections := make([]*pb.CollectionConfig, len(collections))
	for i, c := range collections {
		pbcollections[i] = collectionConfigToPb(c)
	}
	_, err := c.c.NewDBFromAddr(ctx, &pb.NewDBFromAddrRequest{
		DbAddr:      addr.String(),
		DbKey:       key.Bytes(),
		Collections: pbcollections,
	})
	return err
}

func collectionConfigToPb(c db.CollectionConfig) *pb.CollectionConfig {
	idx := make([]*pb.CollectionConfig_IndexConfig, len(c.Indexes))
	for i, index := range c.Indexes {
		idx[i] = &pb.CollectionConfig_IndexConfig{
			Path:   index.Path,
			Unique: index.Unique,
		}
	}
	return &pb.CollectionConfig{
		Name:    c.Name,
		Schema:  c.Schema,
		Indexes: idx,
	}
}

// GetDBInfo retrives db addresses and keys.
func (c *Client) GetDBInfo(ctx context.Context, dbID thread.ID) (*pb.GetDBInfoReply, error) {
	return c.c.GetDBInfo(ctx, &pb.GetDBInfoRequest{
		DbID: dbID.String(),
	})
}

// NewCollection creates a new collection
func (c *Client) NewCollection(ctx context.Context, dbID thread.ID, config db.CollectionConfig) error {
	_, err := c.c.NewCollection(ctx, &pb.NewCollectionRequest{
		DbID:   dbID.String(),
		Config: collectionConfigToPb(config),
	})
	return err
}

// Create creates new instances of objects
func (c *Client) Create(ctx context.Context, dbID thread.ID, collectionName string, items ...interface{}) error {
	values, err := marshalItems(items)
	if err != nil {
		return err
	}

	resp, err := c.c.Create(ctx, &pb.CreateRequest{
		DbID:           dbID.String(),
		CollectionName: collectionName,
		Values:         values,
	})
	if err != nil {
		return err
	}

	for i, instance := range resp.GetInstances() {
		err := json.Unmarshal([]byte(instance), items[i])
		if err != nil {
			return err
		}
	}

	return nil
}

// Save saves existing instances
func (c *Client) Save(ctx context.Context, dbID thread.ID, collectionName string, items ...interface{}) error {
	values, err := marshalItems(items)
	if err != nil {
		return err
	}

	_, err = c.c.Save(ctx, &pb.SaveRequest{
		DbID:           dbID.String(),
		CollectionName: collectionName,
		Values:         values,
	})
	return err
}

// Delete deletes data
func (c *Client) Delete(ctx context.Context, dbID thread.ID, collectionName string, instanceIDs ...string) error {
	_, err := c.c.Delete(ctx, &pb.DeleteRequest{
		DbID:           dbID.String(),
		CollectionName: collectionName,
		InstanceIDs:    instanceIDs,
	})
	return err
}

// Has checks if the specified instances exist
func (c *Client) Has(ctx context.Context, dbID thread.ID, collectionName string, instanceIDs ...string) (bool, error) {
	resp, err := c.c.Has(ctx, &pb.HasRequest{
		DbID:           dbID.String(),
		CollectionName: collectionName,
		InstanceIDs:    instanceIDs,
	})
	if err != nil {
		return false, err
	}
	return resp.GetExists(), nil
}

// Find finds instances by query
func (c *Client) Find(ctx context.Context, dbID thread.ID, collectionName string, query *db.JSONQuery, dummySlice interface{}) (interface{}, error) {
	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	resp, err := c.c.Find(ctx, &pb.FindRequest{
		DbID:           dbID.String(),
		CollectionName: collectionName,
		QueryJSON:      queryBytes,
	})
	if err != nil {
		return nil, err
	}
	return processFindReply(resp, dummySlice)
}

// FindByID finds an instance by id
func (c *Client) FindByID(ctx context.Context, dbID thread.ID, collectionName, instanceID string, instance interface{}) error {
	resp, err := c.c.FindByID(ctx, &pb.FindByIDRequest{
		DbID:           dbID.String(),
		CollectionName: collectionName,
		InstanceID:     instanceID,
	})
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(resp.GetInstance()), instance)
}

// ReadTransaction returns a read transaction that can be started and used and ended
func (c *Client) ReadTransaction(ctx context.Context, dbID thread.ID, collectionName string) (*ReadTransaction, error) {
	client, err := c.c.ReadTransaction(ctx)
	if err != nil {
		return nil, err
	}
	return &ReadTransaction{
		client:         client,
		dbID:           dbID,
		collectionName: collectionName,
	}, nil
}

// WriteTransaction returns a read transaction that can be started and used and ended
func (c *Client) WriteTransaction(ctx context.Context, dbID thread.ID, collectionName string) (*WriteTransaction, error) {
	client, err := c.c.WriteTransaction(ctx)
	if err != nil {
		return nil, err
	}
	return &WriteTransaction{client: client, dbID: dbID, collectionName: collectionName}, nil
}

// Listen provides an update whenever the specified db, collection, or instance is updated
func (c *Client) Listen(ctx context.Context, dbID thread.ID, listenOptions ...ListenOption) (<-chan ListenEvent, error) {
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
		DbID:    dbID.String(),
		Filters: filters,
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

func marshalItems(items []interface{}) ([]string, error) {
	values := make([]string, len(items))
	for i, item := range items {
		bytes, err := json.Marshal(item)
		if err != nil {
			return []string{}, err
		}
		values[i] = string(bytes)
	}
	return values, nil
}
