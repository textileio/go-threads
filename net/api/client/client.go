package client

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipld-format"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/cbor"
	core "github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/crypto"
	"github.com/textileio/go-threads/crypto/symmetric"
	"github.com/textileio/go-threads/net"
	pb "github.com/textileio/go-threads/net/api/pb"
	"github.com/textileio/go-threads/net/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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

func (c *Client) GetHostID(ctx context.Context) (peer.ID, error) {
	resp, err := c.c.GetHostID(ctx, &pb.GetHostIDRequest{})
	if err != nil {
		return "", err
	}
	return peer.IDFromBytes(resp.PeerID)
}

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

func (c *Client) CreateThread(ctx context.Context, id thread.ID, opts ...core.NewThreadOption) (info thread.Info, err error) {
	args := &core.NewThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	keys, err := getThreadKeys(args)
	if err != nil {
		return
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	resp, err := c.c.CreateThread(ctx, &pb.CreateThreadRequest{
		ThreadID: id.Bytes(),
		Keys:     keys,
	})
	if err != nil {
		return
	}
	return threadInfoFromProto(resp)
}

func (c *Client) AddThread(ctx context.Context, addr ma.Multiaddr, opts ...core.NewThreadOption) (info thread.Info, err error) {
	args := &core.NewThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	keys, err := getThreadKeys(args)
	if err != nil {
		return
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	resp, err := c.c.AddThread(ctx, &pb.AddThreadRequest{
		Addr: addr.Bytes(),
		Keys: keys,
	})
	if err != nil {
		return
	}
	return threadInfoFromProto(resp)
}

func (c *Client) GetThread(ctx context.Context, id thread.ID, opts ...core.ThreadOption) (info thread.Info, err error) {
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	resp, err := c.c.GetThread(ctx, &pb.GetThreadRequest{
		ThreadID: id.Bytes(),
	})
	if err != nil {
		return
	}
	return threadInfoFromProto(resp)
}

func (c *Client) PullThread(ctx context.Context, id thread.ID, opts ...core.ThreadOption) error {
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	_, err := c.c.PullThread(ctx, &pb.PullThreadRequest{
		ThreadID: id.Bytes(),
	})
	return err
}

func (c *Client) DeleteThread(ctx context.Context, id thread.ID, opts ...core.ThreadOption) error {
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	_, err := c.c.DeleteThread(ctx, &pb.DeleteThreadRequest{
		ThreadID: id.Bytes(),
	})
	return err
}

func (c *Client) AddReplicator(ctx context.Context, id thread.ID, paddr ma.Multiaddr, opts ...core.ThreadOption) (pid peer.ID, err error) {
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	resp, err := c.c.AddReplicator(ctx, &pb.AddReplicatorRequest{
		ThreadID: id.Bytes(),
		Addr:     paddr.Bytes(),
	})
	if err != nil {
		return
	}
	return peer.IDFromBytes(resp.PeerID)
}

func (c *Client) CreateRecord(ctx context.Context, id thread.ID, body format.Node, opts ...core.ThreadOption) (core.ThreadRecord, error) {
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	info, err := c.GetThread(ctx, id, opts...)
	if err != nil {
		return nil, err
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	resp, err := c.c.CreateRecord(ctx, &pb.CreateRecordRequest{
		ThreadID: id.Bytes(),
		Body:     body.RawData(),
	})
	if err != nil {
		return nil, err
	}
	return threadRecordFromProto(resp, info.Key.Service())
}

func (c *Client) AddRecord(ctx context.Context, id thread.ID, lid peer.ID, rec core.Record, opts ...core.ThreadOption) error {
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	lidb, _ := lid.Marshal()
	prec, err := cbor.RecordToProto(ctx, nil, rec)
	if err != nil {
		return err
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	_, err = c.c.AddRecord(ctx, &pb.AddRecordRequest{
		ThreadID: id.Bytes(),
		LogID:    lidb,
		Record:   util.RecFromServiceRec(prec),
	})
	return err
}

func (c *Client) GetRecord(ctx context.Context, id thread.ID, rid cid.Cid, opts ...core.ThreadOption) (core.Record, error) {
	args := &core.ThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	info, err := c.GetThread(ctx, id, opts...)
	if err != nil {
		return nil, err
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	resp, err := c.c.GetRecord(ctx, &pb.GetRecordRequest{
		ThreadID: id.Bytes(),
		RecordID: rid.Bytes(),
	})
	if err != nil {
		return nil, err
	}
	return cbor.RecordFromProto(util.RecToServiceRec(resp.Record), info.Key.Service())
}

func (c *Client) Subscribe(ctx context.Context, opts ...core.SubOption) (<-chan core.ThreadRecord, error) {
	args := &core.SubOptions{}
	for _, opt := range opts {
		opt(args)
	}
	ids := make([][]byte, len(args.ThreadIDs))
	var err error
	for i, id := range args.ThreadIDs {
		ids[i] = id.Bytes()
	}
	ctx = thread.NewTokenContext(ctx, args.Token)
	stream, err := c.c.Subscribe(ctx, &pb.SubscribeRequest{
		ThreadIDs: ids,
	})
	if err != nil {
		return nil, err
	}
	threads := make(map[thread.ID]*symmetric.Key) // Service-key cache
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
				log.Fatalf("error casting thread ID: %v", err)
			}
			var sk *symmetric.Key
			var ok bool
			if sk, ok = threads[threadID]; !ok {
				info, err := c.GetThread(ctx, threadID, core.WithThreadToken(args.Token))
				if err != nil {
					log.Fatalf("error getting thread: %v", err)
				}
				if info.Key.Service() == nil {
					log.Fatalf("service-key not found")
				}
				sk = info.Key.Service()
				threads[threadID] = sk
			}
			rec, err := threadRecordFromProto(resp, sk)
			if err != nil {
				log.Fatalf("error unpacking record: %v", err)
			}
			channel <- rec
		}
	}()
	return channel, nil
}

func getThreadKeys(args *core.NewThreadOptions) (*pb.Keys, error) {
	keys := &pb.Keys{
		ThreadKey: args.ThreadKey.Bytes(),
	}
	if args.LogKey != nil {
		var err error
		keys.LogKey, err = args.LogKey.Bytes()
		if err != nil {
			return nil, err
		}
	}
	return keys, nil
}

func threadInfoFromProto(reply *pb.ThreadInfoReply) (info thread.Info, err error) {
	threadID, err := thread.Cast(reply.ThreadID)
	if err != nil {
		return
	}
	k, err := thread.KeyFromBytes(reply.ThreadKey)
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
		var head cid.Cid
		if lg.Head != nil {
			head, err = cid.Cast(lg.Head)
			if err != nil {
				return info, err
			}
		} else {
			head = cid.Undef
		}
		logs[i] = thread.LogInfo{
			ID:      id,
			PubKey:  pk,
			PrivKey: sk,
			Addrs:   addrs,
			Head:    head,
		}
	}
	addrs := make([]ma.Multiaddr, len(reply.Addrs))
	for i, addr := range reply.Addrs {
		addrs[i], err = ma.NewMultiaddrBytes(addr)
		if err != nil {
			return info, err
		}
	}
	return thread.Info{
		ID:    threadID,
		Key:   k,
		Logs:  logs,
		Addrs: addrs,
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
	if err = threadID.Validate(); err != nil {
		return nil, err
	}
	return net.NewRecord(rec, threadID, logID), nil
}
