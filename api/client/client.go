package client

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/mr-tron/base58"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-core/crypto/symmetric"
	pb "github.com/textileio/go-threads/api/pb"
	es "github.com/textileio/go-threads/eventstore"
	"github.com/textileio/go-threads/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Client provides the client api
type Client struct {
	client pb.APIClient
	ctx    context.Context
	cancel context.CancelFunc
	conn   *grpc.ClientConn
}

// ListenEvent is used to send data or error values for Listen
type ListenEvent struct {
	data []byte
	err  error
}

// NewClient starts the client
func NewClient(maddr ma.Multiaddr) (*Client, error) {
	addr, err := util.TCPAddrFromMultiAddr(maddr)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	client := &Client{
		client: pb.NewAPIClient(conn),
		ctx:    ctx,
		cancel: cancel,
		conn:   conn,
	}
	return client, nil
}

// Close closes the client's grpc connection and cancels any active requests
func (c *Client) Close() error {
	c.cancel()
	return c.conn.Close()
}

// NewStore cereates a new Store
func (c *Client) NewStore() (string, error) {
	resp, err := c.client.NewStore(c.ctx, &pb.NewStoreRequest{})
	if err != nil {
		return "", err
	}
	return resp.GetID(), nil
}

// RegisterSchema registers a new model shecma
func (c *Client) RegisterSchema(storeID, name, schema string) error {
	req := &pb.RegisterSchemaRequest{
		StoreID: storeID,
		Name:    name,
		Schema:  schema,
	}
	_, err := c.client.RegisterSchema(c.ctx, req)
	return err
}

// Start starts the specified Store
func (c *Client) Start(storeID string) error {
	_, err := c.client.Start(c.ctx, &pb.StartRequest{StoreID: storeID})
	return err
}

// StartFromAddress starts the specified Store with the provided address and keys
func (c *Client) StartFromAddress(storeID string, addr ma.Multiaddr, followKey, readKey *symmetric.Key) error {
	req := &pb.StartFromAddressRequest{
		StoreID:   storeID,
		Address:   addr.String(),
		FollowKey: followKey.Bytes(),
		ReadKey:   readKey.Bytes(),
	}
	_, err := c.client.StartFromAddress(c.ctx, req)
	return err
}

// ModelCreate creates new instances of model objects
func (c *Client) ModelCreate(storeID, modelName string, items ...interface{}) error {
	values, err := marshalItems(items)
	if err != nil {
		return err
	}

	req := &pb.ModelCreateRequest{
		StoreID:   storeID,
		ModelName: modelName,
		Values:    values,
	}

	resp, err := c.client.ModelCreate(c.ctx, req)
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
func (c *Client) ModelSave(storeID, modelName string, items ...interface{}) error {
	values, err := marshalItems(items)
	if err != nil {
		return err
	}

	req := &pb.ModelSaveRequest{
		StoreID:   storeID,
		ModelName: modelName,
		Values:    values,
	}
	_, err = c.client.ModelSave(c.ctx, req)
	return err
}

// ModelDelete deletes data
func (c *Client) ModelDelete(storeID, modelName string, entityIDs ...string) error {
	req := &pb.ModelDeleteRequest{
		StoreID:   storeID,
		ModelName: modelName,
		EntityIDs: entityIDs,
	}
	_, err := c.client.ModelDelete(c.ctx, req)
	return err
}

// GetStoreLink retrives the components required to create a Thread/store invite.
func (c *Client) GetStoreLink(storeID string) ([]string, error) {
	req := &pb.GetStoreLinkRequest{
		StoreID: storeID,
	}
	resp, err := c.client.GetStoreLink(c.ctx, req)
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
func (c *Client) ModelHas(storeID, modelName string, entityIDs ...string) (bool, error) {
	req := &pb.ModelHasRequest{
		StoreID:   storeID,
		ModelName: modelName,
		EntityIDs: entityIDs,
	}
	resp, err := c.client.ModelHas(c.ctx, req)
	if err != nil {
		return false, err
	}
	return resp.GetExists(), nil
}

// ModelFind finds records by query
func (c *Client) ModelFind(storeID, modelName string, query *es.JSONQuery, dummySlice interface{}) (interface{}, error) {
	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	req := &pb.ModelFindRequest{
		StoreID:   storeID,
		ModelName: modelName,
		QueryJSON: queryBytes,
	}
	resp, err := c.client.ModelFind(c.ctx, req)
	if err != nil {
		return nil, err
	}
	return processFindReply(resp, dummySlice)
}

// ModelFindByID finds a record by id
func (c *Client) ModelFindByID(storeID, modelName, entityID string, entity interface{}) error {
	req := &pb.ModelFindByIDRequest{
		StoreID:   storeID,
		ModelName: modelName,
		EntityID:  entityID,
	}
	resp, err := c.client.ModelFindByID(c.ctx, req)
	if err != nil {
		return err
	}
	err = json.Unmarshal([]byte(resp.GetEntity()), entity)
	return err
}

// ReadTransaction returns a read transaction that can be started and used and ended
func (c *Client) ReadTransaction(storeID, modelName string) (*ReadTransaction, error) {
	client, err := c.client.ReadTransaction(c.ctx)
	if err != nil {
		return nil, err
	}
	return &ReadTransaction{client: client, storeID: storeID, modelName: modelName}, nil
}

// WriteTransaction returns a read transaction that can be started and used and ended
func (c *Client) WriteTransaction(storeID, modelName string) (*WriteTransaction, error) {
	client, err := c.client.WriteTransaction(c.ctx)
	if err != nil {
		return nil, err
	}
	return &WriteTransaction{client: client, storeID: storeID, modelName: modelName}, nil
}

// Listen provides an update whenever the specified model is updated
func (c *Client) Listen(storeID, modelName, entityID string) (<-chan ListenEvent, func()) {
	channel := make(chan ListenEvent)
	ctx, cancel := context.WithCancel(c.ctx)
	go func() {
		defer close(channel)
		req := &pb.ListenRequest{
			StoreID:   storeID,
			ModelName: modelName,
			EntityID:  entityID,
		}
		stream, err := c.client.Listen(ctx, req)
		if err != nil {
			channel <- ListenEvent{err: err}
			return
		}
		for {
			event, err := stream.Recv()
			if err != nil {
				stat := status.Convert(err)
				if stat == nil || (stat.Code() != codes.Canceled) {
					channel <- ListenEvent{err: err}
				}
				break
			}
			bytes := []byte(event.GetEntity())
			channel <- ListenEvent{data: bytes}
		}
	}()
	return channel, cancel
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
