package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"

	ma "github.com/multiformats/go-multiaddr"
	pb "github.com/textileio/go-threads/api/pb"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ActionType describes the type of event action when subscribing to data updates.
type ActionType int

const (
	// ActionCreate represents an event for creating a new instance.
	ActionCreate ActionType = iota + 1
	// ActionSave represents an event for saving changes to an existing instance.
	ActionSave
	// ActionDelete represents an event for deleting existing instance.
	ActionDelete
)

// Action represents a data event delivered to a listener.
type Action struct {
	Collection string
	Type       ActionType
	InstanceID string
	Instance   []byte
}

// ListenActionType describes the type of event action when receiving data updates.
type ListenActionType int

const (
	// ListenAll specifies that Create, Save, and Delete events should be listened for.
	ListenAll ListenActionType = iota
	// ListenCreate specifies that Create events should be listened for.
	ListenCreate
	// ListenSave specifies that Save events should be listened for.
	ListenSave
	// ListenDelete specifies that Delete events should be listened for.
	ListenDelete
)

// ListenOption represents a filter to apply when listening for data updates.
type ListenOption struct {
	Type       ListenActionType
	Collection string
	InstanceID string
}

// ListenEvent is used to send data or error values for Listen.
type ListenEvent struct {
	Action Action
	Err    error
}

// Client provides the client api.
type Client struct {
	c    pb.APIClient
	conn *grpc.ClientConn
}

// Instances is a list of collection instances.
type Instances []interface{}

// NewClient starts the client.
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

// Close closes the client's grpc connection and cancels any active requests.
func (c *Client) Close() error {
	return c.conn.Close()
}

// GetToken gets a db token for use with the rest of the API.
func (c *Client) GetToken(ctx context.Context, identity thread.Identity) (tok thread.Token, err error) {
	stream, err := c.c.GetToken(ctx)
	if err != nil {
		return
	}
	defer func() {
		if e := stream.CloseSend(); e != nil && err == nil {
			err = e
		}
	}()
	if err = stream.Send(&pb.GetTokenRequest{
		Payload: &pb.GetTokenRequest_Key{
			Key: identity.GetPublic().String(),
		},
	}); err == io.EOF {
		var noOp interface{}
		return tok, stream.RecvMsg(noOp)
	} else if err != nil {
		return
	}

	rep, err := stream.Recv()
	if err != nil {
		return
	}
	var challenge []byte
	switch payload := rep.Payload.(type) {
	case *pb.GetTokenReply_Challenge:
		challenge = payload.Challenge
	default:
		return tok, fmt.Errorf("challenge was not received")
	}

	sig, err := identity.Sign(ctx, challenge)
	if err != nil {
		return
	}
	if err = stream.Send(&pb.GetTokenRequest{
		Payload: &pb.GetTokenRequest_Signature{
			Signature: sig,
		},
	}); err == io.EOF {
		var noOp interface{}
		return tok, stream.RecvMsg(noOp)
	} else if err != nil {
		return
	}

	rep, err = stream.Recv()
	if err != nil {
		return
	}
	switch payload := rep.Payload.(type) {
	case *pb.GetTokenReply_Token:
		tok = thread.Token(payload.Token)
	default:
		return tok, fmt.Errorf("token was not received")
	}
	return tok, nil
}

// NewDB creates a new DB with ID.
func (c *Client) NewDB(ctx context.Context, dbID thread.ID, opts ...db.NewManagedOption) error {
	args := &db.NewManagedOptions{}
	for _, opt := range opts {
		opt(args)
	}
	pbcollections := make([]*pb.CollectionConfig, len(args.Collections))
	for i, c := range args.Collections {
		cc, err := collectionConfigToPb(c)
		if err != nil {
			return err
		}
		pbcollections[i] = cc
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	_, err := c.c.NewDB(ctx, &pb.NewDBRequest{
		DbID:        dbID.Bytes(),
		Collections: pbcollections,
		Name:        args.Name,
	})
	return err
}

// NewDBFromAddr creates a new DB with address and keys.
func (c *Client) NewDBFromAddr(ctx context.Context, dbAddr ma.Multiaddr, dbKey thread.Key, opts ...db.NewManagedOption) error {
	args := &db.NewManagedOptions{}
	for _, opt := range opts {
		opt(args)
	}
	pbcollections := make([]*pb.CollectionConfig, len(args.Collections))
	for i, c := range args.Collections {
		cc, err := collectionConfigToPb(c)
		if err != nil {
			return err
		}
		pbcollections[i] = cc
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	_, err := c.c.NewDBFromAddr(ctx, &pb.NewDBFromAddrRequest{
		Addr:        dbAddr.Bytes(),
		Key:         dbKey.Bytes(),
		Collections: pbcollections,
		Name:        args.Name,
	})
	return err
}

func collectionConfigToPb(c db.CollectionConfig) (*pb.CollectionConfig, error) {
	idx := make([]*pb.Index, len(c.Indexes))
	for i, index := range c.Indexes {
		idx[i] = &pb.Index{
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

// DBInfo wraps info about a db.
type DBInfo struct {
	Name  string
	Addrs []ma.Multiaddr
	Key   thread.Key
}

// ListDBs lists all dbs.
func (c *Client) ListDBs(ctx context.Context, opts ...db.ManagedOption) (map[thread.ID]*DBInfo, error) {
	args := &db.ManagedOptions{}
	for _, opt := range opts {
		opt(args)
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	res, err := c.c.ListDBs(ctx, &pb.ListDBsRequest{})
	if err != nil {
		return nil, err
	}

	dbs := make(map[thread.ID]*DBInfo)
	for _, d := range res.Dbs {
		id, err := thread.Cast(d.DbID)
		if err != nil {
			return nil, err
		}
		info, err := dbInfoFromPb(d.Info)
		if err != nil {
			return nil, err
		}
		dbs[id] = info
	}
	return dbs, nil
}

// GetDBInfo retrives db addresses and keys.
func (c *Client) GetDBInfo(ctx context.Context, dbID thread.ID, opts ...db.ManagedOption) (*DBInfo, error) {
	args := &db.ManagedOptions{}
	for _, opt := range opts {
		opt(args)
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	res, err := c.c.GetDBInfo(ctx, &pb.GetDBInfoRequest{
		DbID: dbID.Bytes(),
	})
	if err != nil {
		return nil, err
	}
	return dbInfoFromPb(res)
}

func dbInfoFromPb(pi *pb.GetDBInfoReply) (*DBInfo, error) {
	addrs := make([]ma.Multiaddr, len(pi.Addrs))
	for i, bytes := range pi.Addrs {
		addr, err := ma.NewMultiaddrBytes(bytes)
		if err != nil {
			return nil, err
		}
		addrs[i] = addr
	}
	key, err := thread.KeyFromBytes(pi.Key)
	if err != nil {
		return nil, err
	}
	return &DBInfo{Name: pi.Name, Addrs: addrs, Key: key}, nil
}

// DeleteDB deletes a db.
func (c *Client) DeleteDB(ctx context.Context, dbID thread.ID, opts ...db.ManagedOption) error {
	args := &db.ManagedOptions{}
	for _, opt := range opts {
		opt(args)
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	_, err := c.c.DeleteDB(ctx, &pb.DeleteDBRequest{
		DbID: dbID.Bytes(),
	})
	return err
}

// NewCollection creates a new collection.
func (c *Client) NewCollection(ctx context.Context, dbID thread.ID, config db.CollectionConfig, opts ...db.ManagedOption) error {
	args := &db.ManagedOptions{}
	for _, opt := range opts {
		opt(args)
	}
	cc, err := collectionConfigToPb(config)
	if err != nil {
		return err
	}
	_, err = c.c.NewCollection(ctx, &pb.NewCollectionRequest{
		DbID:   dbID.Bytes(),
		Config: cc,
	})
	return err
}

// UpdateCollection updates an existing collection.
func (c *Client) UpdateCollection(ctx context.Context, dbID thread.ID, config db.CollectionConfig, opts ...db.ManagedOption) error {
	args := &db.ManagedOptions{}
	for _, opt := range opts {
		opt(args)
	}
	cc, err := collectionConfigToPb(config)
	if err != nil {
		return err
	}
	_, err = c.c.UpdateCollection(ctx, &pb.UpdateCollectionRequest{
		DbID:   dbID.Bytes(),
		Config: cc,
	})
	return err
}

// DeleteCollection deletes a collection.
func (c *Client) DeleteCollection(ctx context.Context, dbID thread.ID, name string, opts ...db.ManagedOption) error {
	args := &db.ManagedOptions{}
	for _, opt := range opts {
		opt(args)
	}
	_, err := c.c.DeleteCollection(ctx, &pb.DeleteCollectionRequest{
		DbID: dbID.Bytes(),
		Name: name,
	})
	return err
}

// GetCollectionIndexes returns an existing collection's indexes.
func (c *Client) GetCollectionIndexes(ctx context.Context, dbID thread.ID, name string, opts ...db.ManagedOption) ([]db.Index, error) {
	args := &db.ManagedOptions{}
	for _, opt := range opts {
		opt(args)
	}
	resp, err := c.c.GetCollectionIndexes(ctx, &pb.GetCollectionIndexesRequest{
		DbID: dbID.Bytes(),
		Name: name,
	})
	if err != nil {
		return nil, err
	}
	indexes := make([]db.Index, len(resp.Indexes))
	for i, index := range resp.Indexes {
		indexes[i] = db.Index{
			Path:   index.Path,
			Unique: index.Unique,
		}
	}
	return indexes, nil
}

// Create creates new instances of objects.
func (c *Client) Create(ctx context.Context, dbID thread.ID, collectionName string, instances Instances, opts ...db.TxnOption) ([]string, error) {
	args := &db.TxnOptions{}
	for _, opt := range opts {
		opt(args)
	}
	values, err := marshalItems(instances)
	if err != nil {
		return nil, err
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	resp, err := c.c.Create(ctx, &pb.CreateRequest{
		DbID:           dbID.Bytes(),
		CollectionName: collectionName,
		Instances:      values,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetInstanceIDs(), nil
}

// Save saves existing instances.
func (c *Client) Save(ctx context.Context, dbID thread.ID, collectionName string, instances Instances, opts ...db.TxnOption) error {
	args := &db.TxnOptions{}
	for _, opt := range opts {
		opt(args)
	}
	values, err := marshalItems(instances)
	if err != nil {
		return err
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	_, err = c.c.Save(ctx, &pb.SaveRequest{
		DbID:           dbID.Bytes(),
		CollectionName: collectionName,
		Instances:      values,
	})
	return err
}

// Delete deletes data.
func (c *Client) Delete(ctx context.Context, dbID thread.ID, collectionName string, instanceIDs []string, opts ...db.TxnOption) error {
	args := &db.TxnOptions{}
	for _, opt := range opts {
		opt(args)
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	_, err := c.c.Delete(ctx, &pb.DeleteRequest{
		DbID:           dbID.Bytes(),
		CollectionName: collectionName,
		InstanceIDs:    instanceIDs,
	})
	return err
}

// Has checks if the specified instances exist.
func (c *Client) Has(ctx context.Context, dbID thread.ID, collectionName string, instanceIDs []string, opts ...db.TxnOption) (bool, error) {
	args := &db.TxnOptions{}
	for _, opt := range opts {
		opt(args)
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	resp, err := c.c.Has(ctx, &pb.HasRequest{
		DbID:           dbID.Bytes(),
		CollectionName: collectionName,
		InstanceIDs:    instanceIDs,
	})
	if err != nil {
		return false, err
	}
	return resp.GetExists(), nil
}

// Find finds instances by query.
func (c *Client) Find(ctx context.Context, dbID thread.ID, collectionName string, query *db.Query, dummy interface{}, opts ...db.TxnOption) (interface{}, error) {
	args := &db.TxnOptions{}
	for _, opt := range opts {
		opt(args)
	}
	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	resp, err := c.c.Find(ctx, &pb.FindRequest{
		DbID:           dbID.Bytes(),
		CollectionName: collectionName,
		QueryJSON:      queryBytes,
	})
	if err != nil {
		return nil, err
	}
	return processFindReply(resp, dummy)
}

// FindByID finds an instance by id.
func (c *Client) FindByID(ctx context.Context, dbID thread.ID, collectionName, instanceID string, instance interface{}, opts ...db.TxnOption) error {
	args := &db.TxnOptions{}
	for _, opt := range opts {
		opt(args)
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	resp, err := c.c.FindByID(ctx, &pb.FindByIDRequest{
		DbID:           dbID.Bytes(),
		CollectionName: collectionName,
		InstanceID:     instanceID,
	})
	if err != nil {
		return err
	}
	return json.Unmarshal(resp.GetInstance(), instance)
}

// ReadTransaction returns a read transaction that can be started and used and ended.
func (c *Client) ReadTransaction(ctx context.Context, dbID thread.ID, collectionName string, opts ...db.TxnOption) (*ReadTransaction, error) {
	args := &db.TxnOptions{}
	for _, opt := range opts {
		opt(args)
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
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

// WriteTransaction returns a read transaction that can be started and used and ended.
func (c *Client) WriteTransaction(ctx context.Context, dbID thread.ID, collectionName string, opts ...db.TxnOption) (*WriteTransaction, error) {
	args := &db.TxnOptions{}
	for _, opt := range opts {
		opt(args)
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	client, err := c.c.WriteTransaction(ctx)
	if err != nil {
		return nil, err
	}
	return &WriteTransaction{
		client:         client,
		dbID:           dbID,
		collectionName: collectionName,
	}, nil
}

// Listen provides an update whenever the specified db, collection, or instance is updated.
func (c *Client) Listen(ctx context.Context, dbID thread.ID, listenOptions []ListenOption, opts ...db.TxnOption) (<-chan ListenEvent, error) {
	args := &db.TxnOptions{}
	for _, opt := range opts {
		opt(args)
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
	ctx = thread.NewTokenContext(ctx, args.Token)
	stream, err := c.c.Listen(ctx, &pb.ListenRequest{
		DbID:    dbID.Bytes(),
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

func processFindReply(reply *pb.FindReply, dummy interface{}) (interface{}, error) {
	sliceType := reflect.TypeOf(dummy)
	elementType := sliceType.Elem()
	length := len(reply.GetInstances())
	results := reflect.MakeSlice(reflect.SliceOf(sliceType), length, length)
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
