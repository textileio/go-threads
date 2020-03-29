package client

import (
	"context"
	"io"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
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

func (c *Client) CreateThread(ctx context.Context, id thread.ID, opts ...core.NewThreadOption) (info thread.Info, err error) {
	signed, err := signCreds(creds)
	if err != nil {
		return
	}
	args := &core.NewThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	keys, err := getThreadKeys(args)
	if err != nil {
		return
	}
	resp, err := c.c.CreateThread(ctx, &pb.CreateThreadRequest{
		Credentials: signed,
		Keys:        keys,
	})
	if err != nil {
		return
	}
	return threadInfoFromProto(resp)
}

func (c *Client) AddThread(ctx context.Context, addr ma.Multiaddr, opts ...core.NewThreadOption) (info thread.Info, err error) {
	signed, err := signCreds(creds)
	if err != nil {
		return
	}
	args := &core.NewThreadOptions{}
	for _, opt := range opts {
		opt(args)
	}
	keys, err := getThreadKeys(args)
	if err != nil {
		return
	}
	resp, err := c.c.AddThread(ctx, &pb.AddThreadRequest{
		Credentials: signed,
		Addr:        addr.Bytes(),
		Keys:        keys,
	})
	if err != nil {
		return
	}
	return threadInfoFromProto(resp)
}

func (c *Client) GetThread(ctx context.Context, id thread.ID, opts ...core.ThreadOption) (info thread.Info, err error) {
	signed, err := signCreds(creds)
	if err != nil {
		return
	}
	resp, err := c.c.GetThread(ctx, &pb.GetThreadRequest{
		Credentials: signed,
	})
	if err != nil {
		return
	}
	return threadInfoFromProto(resp)
}

func (c *Client) PullThread(ctx context.Context, id thread.ID, opts ...core.ThreadOption) error {
	signed, err := signCreds(creds)
	if err != nil {
		return err
	}
	_, err = c.c.PullThread(ctx, &pb.PullThreadRequest{
		Credentials: signed,
	})
	return err
}

func (c *Client) DeleteThread(ctx context.Context, id thread.ID, opts ...core.ThreadOption) error {
	signed, err := signCreds(creds)
	if err != nil {
		return err
	}
	_, err = c.c.DeleteThread(ctx, &pb.DeleteThreadRequest{
		Credentials: signed,
	})
	return err
}

func (c *Client) AddReplicator(ctx context.Context, id thread.ID, paddr ma.Multiaddr, opts ...core.ThreadOption) (pid peer.ID, err error) {
	signed, err := signCreds(creds)
	if err != nil {
		return
	}
	resp, err := c.c.AddReplicator(ctx, &pb.AddReplicatorRequest{
		Credentials: signed,
		Addr:        paddr.Bytes(),
	})
	if err != nil {
		return
	}
	return peer.IDFromBytes(resp.PeerID)
}

func (c *Client) CreateRecord(ctx context.Context, id thread.ID, body format.Node, opts ...core.ThreadOption) (core.ThreadRecord, error) {
	info, err := c.GetThread(ctx, creds)
	if err != nil {
		return nil, err
	}
	signed, err := signCreds(creds)
	if err != nil {
		return nil, err
	}
	resp, err := c.c.CreateRecord(ctx, &pb.CreateRecordRequest{
		Credentials: signed,
		Body:        body.RawData(),
	})
	if err != nil {
		return nil, err
	}
	return threadRecordFromProto(resp, info.Key.Service())
}

func (c *Client) AddRecord(ctx context.Context, id thread.ID, lid peer.ID, rec core.Record, opts ...core.ThreadOption) error {
	signed, err := signCreds(creds)
	if err != nil {
		return err
	}
	lidb, _ := lid.Marshal()
	prec, err := cbor.RecordToProto(ctx, nil, rec)
	if err != nil {
		return err
	}
	_, err = c.c.AddRecord(ctx, &pb.AddRecordRequest{
		Credentials: signed,
		LogID:       lidb,
		Record:      util.RecFromServiceRec(prec),
	})
	return err
}

func (c *Client) GetRecord(ctx context.Context, id thread.ID, rid cid.Cid, opts ...core.ThreadOption) (core.Record, error) {
	info, err := c.GetThread(ctx, creds)
	if err != nil {
		return nil, err
	}
	signed, err := signCreds(creds)
	if err != nil {
		return nil, err
	}
	resp, err := c.c.GetRecord(ctx, &pb.GetRecordRequest{
		Credentials: signed,
		RecordID:    rid.Bytes(),
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
	creds := make(map[thread.ID]thread.Credentials)
	signed := make([]*pb.Credentials, len(args.Credentials))
	var err error
	for i, c := range args.Credentials {
		signed[i], err = signCreds(c)
		if err != nil {
			return nil, err
		}
		creds[c.ThreadID()] = c
	}
	stream, err := c.c.Subscribe(ctx, &pb.SubscribeRequest{
		Credentials: signed,
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
					log.Errorf("error in subscription stream: %v", err)
				}
				return
			}
			threadID, err := thread.Cast(resp.ThreadID)
			if err != nil {
				log.Errorf("error casting thread ID: %v", err)
				continue
			}
			var sk *symmetric.Key
			var ok bool
			if sk, ok = threads[threadID]; !ok {
				info, err := c.GetThread(ctx, creds[threadID])
				if err != nil {
					log.Errorf("error getting thread: %v", err)
					continue
				}
				if info.Key.Service() == nil {
					log.Error("service-key not found")
					continue
				}
				sk = info.Key.Service()
				threads[threadID] = sk
			}
			rec, err := threadRecordFromProto(resp, sk)
			if err != nil {
				log.Errorf("error unpacking record: %v", err)
				continue
			}
			channel <- rec
		}
	}()
	return channel, nil
}

func signCreds(creds thread.Credentials) (pcreds *pb.Credentials, err error) {
	pcreds = &pb.Credentials{
		ThreadID: creds.ThreadID().Bytes(),
	}
	pk, sig, err := creds.Sign()
	if err != nil {
		return
	}
	if pk == nil {
		return
	}
	pcreds.PubKey, err = ic.MarshalPublicKey(pk)
	if err != nil {
		return
	}
	pcreds.Signature = sig
	return pcreds, nil
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
	return thread.Info{
		ID:   threadID,
		Key:  k,
		Logs: logs,
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
	return net.NewRecord(rec, threadID, logID), nil
}
