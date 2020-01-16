package client

import (
	"context"

	"github.com/textileio/go-threads/crypto/symmetric"

	"github.com/ipfs/go-cid"
	cbornode "github.com/ipfs/go-ipld-cbor"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-threads/cbor"
	core "github.com/textileio/go-threads/core/service"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/service"
	pb "github.com/textileio/go-threads/service/api/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var log = logging.Logger("threadserviceclient")

// Client provides the client api.
type Client struct {
	c    pb.APIClient
	conn *grpc.ClientConn
}

var _ core.API = (*Client)(nil)

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

// GetHostID returns the service's host peer ID.
func (c *Client) GetHostID(ctx context.Context) (peer.ID, error) {
	resp, err := c.c.GetHostID(ctx, &pb.GetHostIDRequest{})
	if err != nil {
		return "", err
	}
	return peer.Decode(resp.PeerID)
}

// CreateThread with id.
func (c *Client) CreateThread(ctx context.Context, id thread.ID) (info thread.Info, err error) {
	resp, err := c.c.CreateThread(ctx, &pb.CreateThreadRequest{
		ThreadID: id.String(),
	})
	if err != nil {
		return
	}
	rk, err := symmetric.NewKey(resp.ReadKey)
	if err != nil {
		return
	}
	fk, err := symmetric.NewKey(resp.FollowKey)
	if err != nil {
		return
	}
	return thread.Info{
		ID:        id,
		FollowKey: fk,
		ReadKey:   rk,
	}, nil
}

func (c *Client) AddThread(ctx context.Context, addr ma.Multiaddr, opts ...core.AddOption) (info thread.Info, err error) {
	args := &core.AddOptions{}
	for _, opt := range opts {
		opt(args)
	}

	resp, err := c.c.AddThread(ctx, &pb.AddThreadRequest{
		Addr:      addr.String(),
		ReadKey:   args.ReadKey.Bytes(),
		FollowKey: args.FollowKey.Bytes(),
	})
	if err != nil {
		return
	}
	threadID, err := thread.Decode(resp.ThreadID)
	if err != nil {
		return
	}
	logIDs := make([]peer.ID, len(resp.LogIDs))
	for i, lid := range resp.LogIDs {
		logIDs[i], err = peer.Decode(lid)
		if err != nil {
			return
		}
	}
	return thread.Info{
		ID:        threadID,
		Logs:      logIDs,
		FollowKey: args.FollowKey,
		ReadKey:   args.ReadKey,
	}, nil
}

func (c *Client) PullThread(ctx context.Context, id thread.ID) error {
	_, err := c.c.PullThread(ctx, &pb.PullThreadRequest{
		ThreadID: id.String(),
	})
	return err
}

func (c *Client) DeleteThread(ctx context.Context, id thread.ID) error {
	_, err := c.c.DeleteThread(ctx, &pb.DeleteThreadRequest{
		ThreadID: id.String(),
	})
	return err
}

func (c *Client) AddFollower(ctx context.Context, id thread.ID, pid peer.ID) error {
	_, err := c.c.AddFollower(ctx, &pb.AddFollowerRequest{
		ThreadID: id.String(),
		PeerID:   pid.String(),
	})
	return err
}

func (c *Client) AddRecord(ctx context.Context, id thread.ID, body format.Node) (core.ThreadRecord, error) {
	resp, err := c.c.AddRecord(ctx, &pb.AddRecordRequest{
		ThreadID: id.String(),
		Body:     body.RawData(),
	})
	if err != nil {
		return nil, err
	}
	return threadRecordFromPbNewRecord(resp)
}

func (c *Client) GetRecord(ctx context.Context, id thread.ID, rid cid.Cid) (core.Record, error) {
	resp, err := c.c.GetRecord(ctx, &pb.GetRecordRequest{
		ThreadID: id.String(),
		RecordID: rid.String(),
	})
	if err != nil {
		return nil, err
	}
	return recordFromPbRecord(resp.Record)
}

func (c *Client) Subscribe(ctx context.Context, opts ...core.SubOption) (<-chan core.ThreadRecord, error) {
	args := &core.SubOptions{}
	for _, opt := range opts {
		opt(args)
	}
	threadIDs := make([]string, len(args.ThreadIDs))
	for i, id := range args.ThreadIDs {
		threadIDs[i] = id.String()
	}
	stream, err := c.c.Subscribe(ctx, &pb.SubscribeRequest{
		ThreadIDs: threadIDs,
	})
	if err != nil {
		return nil, err
	}

	channel := make(chan core.ThreadRecord)
	go func() {
		defer close(channel)
		for {
			resp, err := stream.Recv()
			if err != nil {
				stat := status.Convert(err)
				if stat == nil || (stat.Code() != codes.Canceled) {
					log.Fatal(err)
				}
				return
			}
			rec, err := threadRecordFromPbNewRecord(resp)
			if err != nil {
				log.Fatal(err)
			}
			channel <- rec
		}
	}()
	return channel, nil
}

func recordFromPbRecord(rec *pb.Record) (core.Record, error) {
	rnode, err := cbornode.Decode(rec.Node, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	block, err := cid.Decode(rec.Block)
	if err != nil {
		return nil, err
	}
	prev, err := cid.Decode(rec.Prev)
	if err != nil {
		return nil, err
	}
	return cbor.NewRecord(rnode, block, rec.Sig, prev), nil
}

func threadRecordFromPbNewRecord(reply *pb.NewRecordReply) (core.ThreadRecord, error) {
	threadID, err := thread.Decode(reply.ThreadID)
	if err != nil {
		return nil, err
	}
	logID, err := peer.Decode(reply.LogID)
	if err != nil {
		return nil, err
	}
	rec, err := recordFromPbRecord(reply.Record)
	if err != nil {
		return nil, err
	}
	return service.NewRecord(rec, threadID, logID), nil
}
