// Package api is all about the Network API. It contains the protobuf definition (under /pb), a Go client (under /client) and a gRPC service for the API backed by the threads network.
package api

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	cbornode "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-threads/cbor"
	"github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	pb "github.com/textileio/go-threads/net/api/pb"
	"github.com/textileio/go-threads/net/util"
	tutil "github.com/textileio/go-threads/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	log = logging.Logger("netapi")
)

// Service is a gRPC service for a thread network.
type Service struct {
	net net.Net
}

// Config specifies service settings.
type Config struct {
	Debug bool
}

// NewService starts and returns a new service.
func NewService(network net.Net, conf Config) (*Service, error) {
	var err error
	if conf.Debug {
		err = tutil.SetLogLevels(map[string]logging.LogLevel{
			"netapi": logging.LevelDebug,
		})
		if err != nil {
			return nil, err
		}
	}
	return &Service{net: network}, nil
}

func (s *Service) GetHostID(_ context.Context, _ *pb.GetHostIDRequest) (*pb.GetHostIDReply, error) {
	log.Debugf("received get host ID request")

	return &pb.GetHostIDReply{
		PeerID: marshalPeerID(s.net.Host().ID()),
	}, nil
}

type remoteIdentity struct {
	pk     thread.PubKey
	server pb.API_GetTokenServer
}

func (i *remoteIdentity) MarshalBinary() ([]byte, error) {
	return nil, nil
}

func (i *remoteIdentity) UnmarshalBinary([]byte) error {
	return nil
}

func (i *remoteIdentity) Sign(ctx context.Context, msg []byte) ([]byte, error) {
	if err := i.server.Send(&pb.GetTokenReply{
		Payload: &pb.GetTokenReply_Challenge{
			Challenge: msg,
		},
	}); err != nil {
		return nil, err
	}

	var req *pb.GetTokenRequest
	done := make(chan error)
	go func() {
		defer close(done)
		var err error
		req, err = i.server.Recv()
		if err != nil {
			done <- err
			return
		}
	}()
	select {
	case <-ctx.Done():
		return nil, status.Error(codes.DeadlineExceeded, "Challenge deadline exceeded")
	case err, ok := <-done:
		if ok {
			return nil, err
		}
	}

	var sig []byte
	switch payload := req.Payload.(type) {
	case *pb.GetTokenRequest_Signature:
		sig = payload.Signature
	default:
		return nil, status.Error(codes.InvalidArgument, "Signature is required")
	}
	return sig, nil
}

func (i *remoteIdentity) GetPublic() thread.PubKey {
	return i.pk
}

func (i *remoteIdentity) Decrypt(context.Context, []byte) ([]byte, error) {
	return nil, nil // no-op
}

func (i *remoteIdentity) Equals(thread.Identity) bool {
	return false
}

func (s *Service) GetToken(server pb.API_GetTokenServer) error {
	log.Debugf("received get token request")

	req, err := server.Recv()
	if err != nil {
		return err
	}
	key := &thread.Libp2pPubKey{}
	switch payload := req.Payload.(type) {
	case *pb.GetTokenRequest_Key:
		err = key.UnmarshalString(payload.Key)
		if err != nil {
			return err
		}
	default:
		return status.Error(codes.InvalidArgument, "Key is required")
	}

	identity := &remoteIdentity{
		pk:     key,
		server: server,
	}
	tok, err := s.net.GetToken(server.Context(), identity)
	if err != nil {
		return err
	}
	return server.Send(&pb.GetTokenReply{
		Payload: &pb.GetTokenReply_Token{
			Token: string(tok),
		},
	})
}

func (s *Service) CreateThread(ctx context.Context, req *pb.CreateThreadRequest) (*pb.ThreadInfoReply, error) {
	log.Debugf("received create thread request")

	id, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	opts, err := getKeyOptions(req.Keys)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	opts = append(opts, net.WithNewThreadToken(token))
	info, err := s.net.CreateThread(ctx, id, opts...)
	if err != nil {
		return nil, err
	}
	return threadInfoToProto(info)
}

func (s *Service) AddThread(ctx context.Context, req *pb.AddThreadRequest) (*pb.ThreadInfoReply, error) {
	log.Debugf("received add thread request")

	addr, err := ma.NewMultiaddrBytes(req.Addr)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	opts, err := getKeyOptions(req.Keys)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	opts = append(opts, net.WithNewThreadToken(token))
	info, err := s.net.AddThread(ctx, addr, opts...)
	if err != nil {
		return nil, err
	}
	go func() {
		if err := s.net.PullThread(ctx, info.ID, net.WithThreadToken(token)); err != nil {
			log.Errorf("error pulling thread %s: %s", info.ID, err)
		}
	}()
	return threadInfoToProto(info)
}

func (s *Service) GetThread(ctx context.Context, req *pb.GetThreadRequest) (*pb.ThreadInfoReply, error) {
	log.Debugf("received get thread request")

	id, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	info, err := s.net.GetThread(ctx, id, net.WithThreadToken(token))
	if err != nil {
		return nil, err
	}
	return threadInfoToProto(info)
}

func (s *Service) PullThread(ctx context.Context, req *pb.PullThreadRequest) (*pb.PullThreadReply, error) {
	log.Debugf("received pull thread request")

	id, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.net.PullThread(ctx, id, net.WithThreadToken(token)); err != nil {
		return nil, err
	}
	return &pb.PullThreadReply{}, nil
}

func (s *Service) DeleteThread(ctx context.Context, req *pb.DeleteThreadRequest) (*pb.DeleteThreadReply, error) {
	log.Debugf("received delete thread request")

	id, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.net.DeleteThread(ctx, id, net.WithThreadToken(token)); err != nil {
		return nil, err
	}
	return &pb.DeleteThreadReply{}, nil
}

func (s *Service) AddReplicator(ctx context.Context, req *pb.AddReplicatorRequest) (*pb.AddReplicatorReply, error) {
	log.Debugf("received add replicator request")

	id, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	addr, err := ma.NewMultiaddrBytes(req.Addr)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	pid, err := s.net.AddReplicator(ctx, id, addr, net.WithThreadToken(token))
	if err != nil {
		return nil, err
	}
	return &pb.AddReplicatorReply{
		PeerID: marshalPeerID(pid),
	}, nil
}

func (s *Service) CreateRecord(ctx context.Context, req *pb.CreateRecordRequest) (*pb.NewRecordReply, error) {
	log.Debugf("received create record request")

	id, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	body, err := cbornode.Decode(req.Body, mh.SHA2_256, -1)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	rec, err := s.net.CreateRecord(ctx, id, body, net.WithThreadToken(token))
	if err != nil {
		return nil, err
	}
	prec, err := cbor.RecordToProto(ctx, s.net, rec.Value())
	if err != nil {
		return nil, err
	}
	return &pb.NewRecordReply{
		ThreadID: rec.ThreadID().Bytes(),
		LogID:    marshalPeerID(rec.LogID()),
		Record:   util.RecFromServiceRec(prec),
	}, nil
}

func (s *Service) AddRecord(ctx context.Context, req *pb.AddRecordRequest) (*pb.AddRecordReply, error) {
	log.Debugf("received add record request")

	id, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	info, err := s.net.GetThread(ctx, id, net.WithThreadToken(token))
	if err != nil {
		return nil, err
	}
	logID, err := peer.IDFromBytes(req.LogID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	rec, err := cbor.RecordFromProto(util.RecToServiceRec(req.Record), info.Key.Service())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err = s.net.AddRecord(ctx, id, logID, rec, net.WithThreadToken(token)); err != nil {
		return nil, err
	}
	return &pb.AddRecordReply{}, nil
}

func (s *Service) GetRecord(ctx context.Context, req *pb.GetRecordRequest) (*pb.GetRecordReply, error) {
	log.Debugf("received get record request")

	id, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	rid, err := cid.Cast(req.RecordID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	rec, err := s.net.GetRecord(ctx, id, rid, net.WithThreadToken(token))
	if err != nil {
		return nil, err
	}
	prec, err := cbor.RecordToProto(ctx, s.net, rec)
	if err != nil {
		return nil, err
	}
	return &pb.GetRecordReply{
		Record: util.RecFromServiceRec(prec),
	}, nil
}

func (s *Service) Subscribe(req *pb.SubscribeRequest, server pb.API_SubscribeServer) error {
	log.Debugf("received subscribe request")

	opts := make([]net.SubOption, len(req.ThreadIDs))
	for i, id := range req.ThreadIDs {
		id, err := thread.Cast(id)
		if err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		opts[i] = net.WithSubFilter(id)
	}

	token, err := thread.NewTokenFromMD(server.Context())
	if err != nil {
		return err
	}
	opts = append(opts, net.WithSubToken(token))

	sub, err := s.net.Subscribe(server.Context(), opts...)
	if err != nil {
		return err
	}
	for rec := range sub {
		prec, err := cbor.RecordToProto(server.Context(), s.net, rec.Value())
		if err != nil {
			return err
		}
		if err := server.Send(&pb.NewRecordReply{
			ThreadID: rec.ThreadID().Bytes(),
			LogID:    marshalPeerID(rec.LogID()),
			Record:   util.RecFromServiceRec(prec),
		}); err != nil {
			return err
		}
	}
	return nil
}

func marshalPeerID(id peer.ID) []byte {
	b, _ := id.Marshal() // This will never return an error
	return b
}

func getKeyOptions(keys *pb.Keys) (opts []net.NewThreadOption, err error) {
	if keys == nil {
		return
	}
	if keys.ThreadKey != nil {
		k, err := thread.KeyFromBytes(keys.ThreadKey)
		if err != nil {
			return nil, fmt.Errorf("invalid thread-key: %v", err)
		}
		opts = append(opts, net.WithThreadKey(k))
	}
	var lk crypto.Key
	if keys.LogKey != nil {
		lk, err = crypto.UnmarshalPrivateKey(keys.LogKey)
		if err != nil {
			lk, err = crypto.UnmarshalPublicKey(keys.LogKey)
			if err != nil {
				return nil, fmt.Errorf("invalid log-key")
			}
		}
		opts = append(opts, net.WithLogKey(lk))
	}
	return opts, nil
}

func threadInfoToProto(info thread.Info) (*pb.ThreadInfoReply, error) {
	logs := make([]*pb.LogInfo, len(info.Logs))
	for i, lg := range info.Logs {
		pk, err := crypto.MarshalPublicKey(lg.PubKey)
		if err != nil {
			return nil, err
		}
		var sk []byte
		if lg.PrivKey != nil {
			sk, err = crypto.MarshalPrivateKey(lg.PrivKey)
			if err != nil {
				return nil, err
			}
		}
		addrs := make([][]byte, len(lg.Addrs))
		for j, addr := range lg.Addrs {
			addrs[j] = addr.Bytes()
		}
		logs[i] = &pb.LogInfo{
			ID:      marshalPeerID(lg.ID),
			PubKey:  pk,
			PrivKey: sk,
			Addrs:   addrs,
			Head:    lg.Head.Bytes(),
		}
	}
	addrs := make([][]byte, len(info.Addrs))
	for i, addr := range info.Addrs {
		addrs[i] = addr.Bytes()
	}
	return &pb.ThreadInfoReply{
		ThreadID:  info.ID.Bytes(),
		ThreadKey: info.Key.Bytes(),
		Logs:      logs,
		Addrs:     addrs,
	}, nil
}
