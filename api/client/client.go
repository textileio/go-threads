package client

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/mr-tron/base58"
	ma "github.com/multiformats/go-multiaddr"
	pb "github.com/textileio/go-threads/api/pb"
	"github.com/textileio/go-threads/crypto/symmetric"
	"github.com/textileio/go-threads/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ActionType describes the type of event action when subscribing to data updates
type ActionType int

const (
	// ActionCreate represents an event for creating a new entity
	ActionCreate ActionType = iota + 1
	// ActionSave represents an event for saving changes to an existing entity
	ActionSave
	// ActionDelete represents an event for deleting existing entity
	ActionDelete
)

// Action represents a data event delivered to a listener
type Action struct {
	Model    string
	Type     ActionType
	EntityID string
	Entity   []byte
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
	Type     ListenActionType
	Model    string
	EntityID string
}

// ListenEvent is used to send data or error values for Listen
type ListenEvent struct {
	action Action
	err    error
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

// NewStore cereates a new Store
func (c *Client) NewStore(ctx context.Context) (string, error) {
	resp, err := c.c.NewStore(ctx, &pb.NewStoreRequest{})
	if err != nil {
		return "", err
	}
	return resp.GetID(), nil
}

// RegisterSchema registers a new model shecma
func (c *Client) RegisterSchema(ctx context.Context, storeID, name, schema string, indexes ...*store.IndexConfig) error {
	idx := make([]*pb.RegisterSchemaRequest_IndexConfig, len(indexes))
	for i, index := range indexes {
		idx[i] = &pb.RegisterSchemaRequest_IndexConfig{
			Path:   index.Path,
			Unique: index.Unique,
		}
	}
	req := &pb.RegisterSchemaRequest{
		StoreID: storeID,
		Name:    name,
		Schema:  schema,
		Indexes: idx,
	}
	_, err := c.c.RegisterSchema(ctx, req)
	return err
}

// Start starts the specified Store
func (c *Client) Start(ctx context.Context, storeID string) error {
	_, err := c.c.Start(ctx, &pb.StartRequest{StoreID: storeID})
	return err
}

// StartFromAddress starts the specified Store with the provided address and keys
func (c *Client) StartFromAddress(ctx context.Context, storeID string, addr ma.Multiaddr, followKey, readKey *symmetric.Key) error {
	req := &pb.StartFromAddressRequest{
		StoreID:   storeID,
		Address:   addr.String(),
		FollowKey: followKey.Bytes(),
		ReadKey:   readKey.Bytes(),
	}
	_, err := c.c.StartFromAddress(ctx, req)
	return err
}

// ModelCreate creates new instances of model objects
func (c *Client) ModelCreate(ctx context.Context, storeID, modelName string, items ...interface{}) error {
	values, err := marshalItems(items)
	if err != nil {
		return err
	}

	req := &pb.ModelCreateRequest{
		StoreID:   storeID,
		ModelName: modelName,
		Values:    values,
	}

	resp, err := c.c.ModelCreate(ctx, req)
	if err != nil {
		return err
	}

	for i, entity := range resp.GetEntities() {
		err := json.Unmarshal([]byte(entity), items[i])
		if err != nil {
			return err
		}
	}

	return nil
}

// ModelSave saves existing instances
func (c *Client) ModelSave(ctx context.Context, storeID, modelName string, items ...interface{}) error {
	values, err := marshalItems(items)
	if err != nil {
		return err
	}

	req := &pb.ModelSaveRequest{
		StoreID:   storeID,
		ModelName: modelName,
		Values:    values,
	}
	_, err = c.c.ModelSave(ctx, req)
	return err
}

// ModelDelete deletes data
func (c *Client) ModelDelete(ctx context.Context, storeID, modelName string, entityIDs ...string) error {
	req := &pb.ModelDeleteRequest{
		StoreID:   storeID,
		ModelName: modelName,
		EntityIDs: entityIDs,
	}
	_, err := c.c.ModelDelete(ctx, req)
	return err
}

// GetStoreLink retrives the components required to create a Thread/store invite.
func (c *Client) GetStoreLink(ctx context.Context, storeID string) ([]string, error) {
	req := &pb.GetStoreLinkRequest{
		StoreID: storeID,
	}
	resp, err := c.c.GetStoreLink(ctx, req)
	if err != nil {
		return nil, err
	}
	addrs := resp.GetAddresses()
	res := make([]string, len(addrs))
	for i := range addrs {
		addr := addrs[i]
		fKey := base58.Encode(resp.GetFollowKey())
		rKey := base58.Encode(resp.GetReadKey())
		res[i] = addr + "?" + fKey + "&" + rKey
	}
	return res, nil
}

// ModelHas checks if the specified entities exist
func (c *Client) ModelHas(ctx context.Context, storeID, modelName string, entityIDs ...string) (bool, error) {
	req := &pb.ModelHasRequest{
		StoreID:   storeID,
		ModelName: modelName,
		EntityIDs: entityIDs,
	}
	resp, err := c.c.ModelHas(ctx, req)
	if err != nil {
		return false, err
	}
	return resp.GetExists(), nil
}

// ModelFind finds records by query
func (c *Client) ModelFind(ctx context.Context, storeID, modelName string, query *store.JSONQuery, dummySlice interface{}) (interface{}, error) {
	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	req := &pb.ModelFindRequest{
		StoreID:   storeID,
		ModelName: modelName,
		QueryJSON: queryBytes,
	}
	resp, err := c.c.ModelFind(ctx, req)
	if err != nil {
		return nil, err
	}
	return processFindReply(resp, dummySlice)
}

// ModelFindByID finds a record by id
func (c *Client) ModelFindByID(ctx context.Context, storeID, modelName, entityID string, entity interface{}) error {
	req := &pb.ModelFindByIDRequest{
		StoreID:   storeID,
		ModelName: modelName,
		EntityID:  entityID,
	}
	resp, err := c.c.ModelFindByID(ctx, req)
	if err != nil {
		return err
	}
	err = json.Unmarshal([]byte(resp.GetEntity()), entity)
	return err
}

// ReadTransaction returns a read transaction that can be started and used and ended
func (c *Client) ReadTransaction(ctx context.Context, storeID, modelName string) (*ReadTransaction, error) {
	client, err := c.c.ReadTransaction(ctx)
	if err != nil {
		return nil, err
	}
	return &ReadTransaction{client: client, storeID: storeID, modelName: modelName}, nil
}

// WriteTransaction returns a read transaction that can be started and used and ended
func (c *Client) WriteTransaction(ctx context.Context, storeID, modelName string) (*WriteTransaction, error) {
	client, err := c.c.WriteTransaction(ctx)
	if err != nil {
		return nil, err
	}
	return &WriteTransaction{client: client, storeID: storeID, modelName: modelName}, nil
}

// Listen provides an update whenever the specified store, model type, or model instance is updated
func (c *Client) Listen(ctx context.Context, storeID string, listenOptions ...ListenOption) (<-chan ListenEvent, error) {
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
			ModelName: listenOption.Model,
			EntityID:  listenOption.EntityID,
			Action:    action,
		}
	}
	req := &pb.ListenRequest{
		StoreID: storeID,
		Filters: filters,
	}
	stream, err := c.c.Listen(ctx, req)
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
					channel <- ListenEvent{err: err}
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
				channel <- ListenEvent{err: fmt.Errorf("unknown listen reply action %v", event.GetAction())}
				break loop
			}
			action := Action{
				Model:    event.GetModelName(),
				Type:     actionType,
				EntityID: event.GetEntityID(),
				Entity:   event.GetEntity(),
			}
			channel <- ListenEvent{action: action}
		}
	}()
	return channel, nil
}

func processFindReply(reply *pb.ModelFindReply, dummySlice interface{}) (interface{}, error) {
	sliceType := reflect.TypeOf(dummySlice)
	elementType := sliceType.Elem().Elem()
	length := len(reply.GetEntities())
	results := reflect.MakeSlice(sliceType, length, length)
	for i, result := range reply.GetEntities() {
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
