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
	"github.com/textileio/go-threads/db"
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

// NewDB cereates a new DB
func (c *Client) NewDB(ctx context.Context) (string, error) {
	resp, err := c.c.NewDB(ctx, &pb.NewDBRequest{})
	if err != nil {
		return "", err
	}
	return resp.GetID(), nil
}

// NewCollection registers a new model shecma
func (c *Client) NewCollection(ctx context.Context, dbID, name, schema string, indexes ...*db.IndexConfig) error {
	idx := make([]*pb.NewCollectionRequest_IndexConfig, len(indexes))
	for i, index := range indexes {
		idx[i] = &pb.NewCollectionRequest_IndexConfig{
			Path:   index.Path,
			Unique: index.Unique,
		}
	}
	req := &pb.NewCollectionRequest{
		DBID:    dbID,
		Name:    name,
		Schema:  schema,
		Indexes: idx,
	}
	_, err := c.c.NewCollection(ctx, req)
	return err
}

// Start starts the specified DB
func (c *Client) Start(ctx context.Context, dbID string) error {
	_, err := c.c.Start(ctx, &pb.StartRequest{DBID: dbID})
	return err
}

// StartFromAddress starts the specified DB with the provided address and keys
func (c *Client) StartFromAddress(ctx context.Context, dbID string, addr ma.Multiaddr, followKey, readKey *symmetric.Key) error {
	req := &pb.StartFromAddressRequest{
		DBID:      dbID,
		Address:   addr.String(),
		FollowKey: followKey.Bytes(),
		ReadKey:   readKey.Bytes(),
	}
	_, err := c.c.StartFromAddress(ctx, req)
	return err
}

// ModelCreate creates new instances of model objects
func (c *Client) ModelCreate(ctx context.Context, dbID, modelName string, items ...interface{}) error {
	values, err := marshalItems(items)
	if err != nil {
		return err
	}

	req := &pb.ModelCreateRequest{
		DBID:      dbID,
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
func (c *Client) ModelSave(ctx context.Context, dbID, modelName string, items ...interface{}) error {
	values, err := marshalItems(items)
	if err != nil {
		return err
	}

	req := &pb.ModelSaveRequest{
		DBID:      dbID,
		ModelName: modelName,
		Values:    values,
	}
	_, err = c.c.ModelSave(ctx, req)
	return err
}

// ModelDelete deletes data
func (c *Client) ModelDelete(ctx context.Context, dbID, modelName string, entityIDs ...string) error {
	req := &pb.ModelDeleteRequest{
		DBID:      dbID,
		ModelName: modelName,
		EntityIDs: entityIDs,
	}
	_, err := c.c.ModelDelete(ctx, req)
	return err
}

// GetDBLink retrives the components required to create a Thread/db invite.
func (c *Client) GetDBLink(ctx context.Context, dbID string) ([]string, error) {
	req := &pb.GetDBLinkRequest{
		DBID: dbID,
	}
	resp, err := c.c.GetDBLink(ctx, req)
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
func (c *Client) ModelHas(ctx context.Context, dbID, modelName string, entityIDs ...string) (bool, error) {
	req := &pb.ModelHasRequest{
		DBID:      dbID,
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
func (c *Client) ModelFind(ctx context.Context, dbID, modelName string, query *db.JSONQuery, dummySlice interface{}) (interface{}, error) {
	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	req := &pb.ModelFindRequest{
		DBID:      dbID,
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
func (c *Client) ModelFindByID(ctx context.Context, dbID, modelName, entityID string, entity interface{}) error {
	req := &pb.ModelFindByIDRequest{
		DBID:      dbID,
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
func (c *Client) ReadTransaction(ctx context.Context, dbID, modelName string) (*ReadTransaction, error) {
	client, err := c.c.ReadTransaction(ctx)
	if err != nil {
		return nil, err
	}
	return &ReadTransaction{client: client, dbID: dbID, modelName: modelName}, nil
}

// WriteTransaction returns a read transaction that can be started and used and ended
func (c *Client) WriteTransaction(ctx context.Context, dbID, modelName string) (*WriteTransaction, error) {
	client, err := c.c.WriteTransaction(ctx)
	if err != nil {
		return nil, err
	}
	return &WriteTransaction{client: client, dbID: dbID, modelName: modelName}, nil
}

// Listen provides an update whenever the specified db, model type, or model instance is updated
func (c *Client) Listen(ctx context.Context, dbID string, listenOptions ...ListenOption) (<-chan ListenEvent, error) {
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
		DBID:    dbID,
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
				Model:    event.GetModelName(),
				Type:     actionType,
				EntityID: event.GetEntityID(),
				Entity:   event.GetEntity(),
			}
			channel <- ListenEvent{Action: action}
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
