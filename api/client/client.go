package client

import (
	"context"
	"fmt"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-core/crypto/symmetric"
	pb "github.com/textileio/go-textile-threads/api/pb"
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
	return resp.GetID(), err
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
func (c *Client) ModelCreate(storeID, modelName string, values []string) ([]string, error) {
	req := &pb.ModelCreateRequest{
		StoreID:   storeID,
		ModelName: modelName,
		Values:    values,
	}
	resp, err := c.client.ModelCreate(c.ctx, req)
	return resp.GetEntities(), err
}

// ModelSave saves existing instances
func (c *Client) ModelSave(storeID, modelName string, values []string) error {
	req := &pb.ModelSaveRequest{
		StoreID:   storeID,
		ModelName: modelName,
		Values:    values,
	}
	_, err := c.client.ModelSave(c.ctx, req)
	return err
}

// ModelDelete deletes data
func (c *Client) ModelDelete(storeID, modelName string, entityIds []string) error {
	req := &pb.ModelDeleteRequest{
		StoreID:   storeID,
		ModelName: modelName,
		EntityIDs: entityIds,
	}
	_, err := c.client.ModelDelete(c.ctx, req)
	return err
}
