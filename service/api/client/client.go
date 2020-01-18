package client

import (
	"context"
	"io"

	"github.com/textileio/go-threads/service/util"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/cbor"
	core "github.com/textileio/go-threads/core/service"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/crypto"
	"github.com/textileio/go-threads/crypto/symmetric"
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
	return peer.IDFromBytes(resp.PeerID)
}

func (c *Client) CreateThread(ctx context.Context, id thread.ID, opts ...core.KeyOption) (info thread.Info, err error) {
	args := &core.KeyOptions{}
	for _, opt := range opts {
		opt(args)
	}
	req := &pb.CreateThreadRequest{
		ThreadID: id.Bytes(),
		Keys:     &pb.ThreadKeys{},
	}
	if args.FollowKey != nil {
		req.Keys.FollowKey = args.FollowKey.Bytes()
	}
	if args.ReadKey != nil {
		req.Keys.ReadKey = args.ReadKey.Bytes()
	}
	if args.LogKey != nil {
		req.Keys.LogKey, err = args.LogKey.Bytes()
		if err != nil {
			return
		}
	}
	resp, err := c.c.CreateThread(ctx, req)
	if err != nil {
		return
	}
	return threadInfoFromProto(resp)
}

func (c *Client) AddThread(ctx context.Context, addr ma.Multiaddr, opts ...core.KeyOption) (info thread.Info, err error) {
	args := &core.KeyOptions{}
	for _, opt := range opts {
		opt(args)
	}
	req := &pb.AddThreadRequest{
		Addr: addr.Bytes(),
		Keys: &pb.ThreadKeys{},
	}
	if args.FollowKey != nil {
		req.Keys.FollowKey = args.FollowKey.Bytes()
	}
	if args.ReadKey != nil {
		req.Keys.ReadKey = args.ReadKey.Bytes()
	}
	if args.LogKey != nil {
		req.Keys.LogKey, err = args.LogKey.Bytes()
		if err != nil {
			return
		}
	}
	resp, err := c.c.AddThread(ctx, req)
	if err != nil {
		return
	}
	return threadInfoFromProto(resp)
}

func (c *Client) GetThread(ctx context.Context, id thread.ID) (info thread.Info, err error) {
	resp, err := c.c.GetThread(ctx, &pb.GetThreadRequest{
		ThreadID: id.Bytes(),
	})
	if err != nil {
		return
	}
	return threadInfoFromProto(resp)
}

func (c *Client) PullThread(ctx context.Context, id thread.ID) error {
	_, err := c.c.PullThread(ctx, &pb.PullThreadRequest{
		ThreadID: id.Bytes(),
	})
	return err
}

func (c *Client) DeleteThread(ctx context.Context, id thread.ID) error {
	_, err := c.c.DeleteThread(ctx, &pb.DeleteThreadRequest{
		ThreadID: id.Bytes(),
	})
	return err
}

func (c *Client) AddFollower(ctx context.Context, id thread.ID, paddr ma.Multiaddr) (peer.ID, error) {
	resp, err := c.c.AddFollower(ctx, &pb.AddFollowerRequest{
		ThreadID: id.Bytes(),
		Addr:     paddr.Bytes(),
	})
	if err != nil {
		return "", err
	}
	return peer.IDFromBytes(resp.PeerID)
}

func (c *Client) CreateRecord(ctx context.Context, id thread.ID, body format.Node) (core.ThreadRecord, error) {
	info, err := c.GetThread(ctx, id)
	if err != nil {
		return nil, err
	}
	resp, err := c.c.CreateRecord(ctx, &pb.CreateRecordRequest{
		ThreadID: id.Bytes(),
		Body:     body.RawData(),
	})
	if err != nil {
		return nil, err
	}
	return threadRecordFromProto(resp, info.FollowKey)
}

func (c *Client) AddRecord(ctx context.Context, id thread.ID, lid peer.ID, rec core.Record) error {
	lidb, _ := lid.Marshal()
	prec, err := cbor.RecordToProto(ctx, nil, rec)
	if err != nil {
		return err
	}
	_, err = c.c.AddRecord(ctx, &pb.AddRecordRequest{
		ThreadID: id.Bytes(),
		LogID:    lidb,
		Record:   util.RecFromServiceRec(prec),
	})
	return err
}

func (c *Client) GetRecord(ctx context.Context, id thread.ID, rid cid.Cid) (core.Record, error) {
	info, err := c.GetThread(ctx, id)
	if err != nil {
		return nil, err
	}
	resp, err := c.c.GetRecord(ctx, &pb.GetRecordRequest{
		ThreadID: id.Bytes(),
		RecordID: rid.Bytes(),
	})
	if err != nil {
		return nil, err
	}
	return cbor.RecordFromProto(util.RecToServiceRec(resp.Record), info.FollowKey)
}

func (c *Client) Subscribe(ctx context.Context, opts ...core.SubOption) (<-chan core.ThreadRecord, error) {
	args := &core.SubOptions{}
	for _, opt := range opts {
		opt(args)
	}
	threadIDs := make([][]byte, len(args.ThreadIDs))
	for i, id := range args.ThreadIDs {
		threadIDs[i] = id.Bytes()
	}
	stream, err := c.c.Subscribe(ctx, &pb.SubscribeRequest{
		ThreadIDs: threadIDs,
	})
	if err != nil {
		return nil, err
	}
	threads := make(map[thread.ID]*symmetric.Key) // Follow-key cache
	channel := make(chan core.ThreadRecord)
	go func() {
		defer close(channel)
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				stat := status.Convert(err)
				if stat.Code() != codes.Canceled {
					log.Fatalf("error in subscription stream: %v", err)
				}
				return
			}
			threadID, err := thread.Cast(resp.ThreadID)
			if err != nil {
				log.Fatal(err)
			}
			var fk *symmetric.Key
			var ok bool
			if fk, ok = threads[threadID]; !ok {
				info, err := c.GetThread(ctx, threadID)
				if err != nil {
					log.Fatal(err)
				}
				fk = info.FollowKey
				threads[threadID] = fk
			}
			rec, err := threadRecordFromProto(resp, fk)
			if err != nil {
				log.Fatal(err)
			}
			channel <- rec
		}
	}()
	return channel, nil
}

func threadInfoFromProto(reply *pb.ThreadInfoReply) (info thread.Info, err error) {
	threadID, err := thread.Cast(reply.ID)
	if err != nil {
		return
	}
	var rk *symmetric.Key
	if reply.ReadKey != nil {
		rk, err = symmetric.NewKey(reply.ReadKey)
		if err != nil {
			return
		}
	}
	fk, err := symmetric.NewKey(reply.FollowKey)
	if err != nil {
		return
	}
	logs := make([]thread.LogInfo, len(reply.Logs))
	for i, lg := range reply.Logs {
		id, err := peer.IDFromBytes(lg.ID)
		if err != nil {
			return info, err
		}
		pk, err := ic.UnmarshalPublicKey(lg.PubKey)
		if err != nil {
			return info, err
		}
		var sk ic.PrivKey
		if lg.PrivKey != nil {
			sk, err = ic.UnmarshalPrivateKey(lg.PrivKey)
			if err != nil {
				return info, err
			}
		}
		addrs := make([]ma.Multiaddr, len(lg.Addrs))
		for j, addr := range lg.Addrs {
			addrs[j], err = ma.NewMultiaddrBytes(addr)
			if err != nil {
				return info, err
			}
		}
		heads := make([]cid.Cid, len(lg.Heads))
		for k, head := range lg.Heads {
			heads[k], err = cid.Cast(head)
			if err != nil {
				return info, err
			}
		}
		logs[i] = thread.LogInfo{
			ID:      id,
			PubKey:  pk,
			PrivKey: sk,
			Addrs:   addrs,
			Heads:   heads,
		}
	}
	return thread.Info{
		ID:        threadID,
		Logs:      logs,
		FollowKey: fk,
		ReadKey:   rk,
	}, nil
}

func threadRecordFromProto(reply *pb.NewRecordReply, key crypto.DecryptionKey) (core.ThreadRecord, error) {
	threadID, err := thread.Cast(reply.ThreadID)
	if err != nil {
		return nil, err
	}
	logID, err := peer.IDFromBytes(reply.LogID)
	if err != nil {
		return nil, err
	}
	rec, err := cbor.RecordFromProto(util.RecToServiceRec(reply.Record), key)
	if err != nil {
		return nil, err
	}
	return service.NewRecord(rec, threadID, logID), nil
}
