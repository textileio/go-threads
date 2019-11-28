package client

import (
	"context"
	"encoding/json"
	"fmt"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-core/crypto/symmetric"
	pb "github.com/textileio/go-textile-threads/api/pb"
	es "github.com/textileio/go-textile-threads/eventstore"
	"google.golang.org/grpc"
)

// Client provides the client api
type Client struct {
	client pb.APIClient
	ctx    context.Context
}

// NewClient starts the client
func NewClient(host string, port int) (*Client, error) {
	url := fmt.Sprintf("%v:%v", host, port)
	conn, err := grpc.Dial(url, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	client := &Client{
		client: pb.NewAPIClient(conn),
		ctx:    context.Background(),
	}
	return client, nil
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
	// TODO: marshalItems is broken it seems
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
func (c *Client) ModelFind(storeID, modelName string, query es.JSONQuery) ([][]byte, error) {
	queryBytes, err := json.Marshal(query)
	if err != nil {
		return [][]byte{}, err
	}
	req := &pb.ModelFindRequest{
		StoreID:   storeID,
		ModelName: modelName,
		QueryJSON: queryBytes,
	}
	resp, err := c.client.ModelFind(c.ctx, req)
	if err != nil {
		return [][]byte{}, err
	}
	return resp.GetEntities(), nil
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
func (c *Client) Listen(storeID, modelName, entityID string, dummy interface{}) (chan interface{}, error) {
	req := &pb.ListenRequest{
		StoreID:   storeID,
		ModelName: modelName,
		EntityID:  entityID,
	}
	stream, err := c.client.Listen(c.ctx, req)
	if err != nil {
		return nil, err
	}
	channel := make(chan interface{})
	go func() {
		for {
			event, err := stream.Recv()
			if err != nil {
				break
			}
			err = json.Unmarshal([]byte(event.GetEntity()), dummy)
			channel <- dummy
		}
		close(channel)
	}()
	return channel, nil
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
	// TODO: this is returning weird looking data
	return values, nil
}
