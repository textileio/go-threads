package client

import (
	"context"
	"fmt"
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
func (c *Client) RegisterSchema(storeID string, name string, schema string) error {
	req := &pb.RegisterSchemaRequest{
		StoreID: storeID,
		Name:    name,
		Schema:  schema,
	}
	_, err := c.client.RegisterSchema(c.ctx, req)
	return err
}

// ModelCreate creates new instances of model objects
func (c *Client) ModelCreate(storeID string, modelName string, values []string) ([]string, error) {
	req := &pb.ModelCreateRequest{
		StoreID:   storeID,
		ModelName: modelName,
		Values:    values,
	}
	resp, err := c.client.ModelCreate(c.ctx, req)
	return resp.GetEntities(), err
}
