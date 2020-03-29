package api

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/proto"
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
	log = logging.Logger("threadserviceapi")
)

// Service is a gRPC service for a thread network.
type Service struct {
	net net.Net
}

// Config specifies server settings.
type Config struct {
	Debug bool
}

// NewService starts and returns a new service.
func NewService(network net.Net, conf Config) (*Service, error) {
	var err error
	if conf.Debug {
		err = tutil.SetLogLevels(map[string]logging.LogLevel{
			"threadserviceapi": logging.LevelDebug,
		})
		if err != nil {
			return nil, err
		}
	}
	return &Service{net: network}, nil
}

// NewStore adds a new store into the manager.
func (s *Service) GetHostID(_ context.Context, _ *pb.GetHostIDRequest) (*pb.GetHostIDReply, error) {
	log.Debugf("received get host ID request")

	return &pb.GetHostIDReply{
		PeerID: marshalPeerID(s.net.Host().ID()),
	}, nil
}

func (s *Service) CreateThread(ctx context.Context, req *pb.CreateThreadRequest) (*pb.ThreadInfoReply, error) {
	log.Debugf("received create thread request")

	id, err := thread.Cast(req.Body.ThreadID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	opts, err := getKeyOptions(req.Body.Keys)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	opts = append(opts, net.WithNewThreadAuth(auth))
	info, err := s.net.CreateThread(ctx, id, opts...)
	if err != nil {
		return nil, err
	}
	return threadInfoToProto(info)
}

func (s *Service) AddThread(ctx context.Context, req *pb.AddThreadRequest) (*pb.ThreadInfoReply, error) {
	log.Debugf("received add thread request")

	addr, err := ma.NewMultiaddrBytes(req.Body.Addr)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	opts, err := getKeyOptions(req.Body.Keys)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	opts = append(opts, net.WithNewThreadAuth(auth))
	info, err := s.net.AddThread(ctx, addr, opts...)
	if err != nil {
		return nil, err
	}
	go func() {
		if err := s.net.PullThread(ctx, info.ID, net.WithThreadAuth(auth)); err != nil {
			log.Errorf("error pulling thread %s: %s", info.ID, err)
		}
	}()
	return threadInfoToProto(info)
}

func (s *Service) GetThread(ctx context.Context, req *pb.GetThreadRequest) (*pb.ThreadInfoReply, error) {
	log.Debugf("received get thread request")

	id, err := thread.Cast(req.Body.ThreadID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	info, err := s.net.GetThread(ctx, id, net.WithThreadAuth(auth))
	if err != nil {
		return nil, err
	}
	return threadInfoToProto(info)
}

func (s *Service) PullThread(ctx context.Context, req *pb.PullThreadRequest) (*pb.PullThreadReply, error) {
	log.Debugf("received pull thread request")

	id, err := thread.Cast(req.Body.ThreadID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	if err = s.net.PullThread(ctx, id, net.WithThreadAuth(auth)); err != nil {
		return nil, err
	}
	return &pb.PullThreadReply{}, nil
}

func (s *Service) DeleteThread(ctx context.Context, req *pb.DeleteThreadRequest) (*pb.DeleteThreadReply, error) {
	log.Debugf("received delete thread request")

	id, err := thread.Cast(req.Body.ThreadID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	if err := s.net.DeleteThread(ctx, id, net.WithThreadAuth(auth)); err != nil {
		return nil, err
	}
	return &pb.DeleteThreadReply{}, nil
}

func (s *Service) AddReplicator(ctx context.Context, req *pb.AddReplicatorRequest) (*pb.AddReplicatorReply, error) {
	log.Debugf("received add replicator request")

	id, err := thread.Cast(req.Body.ThreadID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	addr, err := ma.NewMultiaddrBytes(req.Body.Addr)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	pid, err := s.net.AddReplicator(ctx, id, addr, net.WithThreadAuth(auth))
	if err != nil {
		return nil, err
	}
	return &pb.AddReplicatorReply{
		PeerID: marshalPeerID(pid),
	}, nil
}

func (s *Service) CreateRecord(ctx context.Context, req *pb.CreateRecordRequest) (*pb.NewRecordReply, error) {
	log.Debugf("received create record request")

	id, err := thread.Cast(req.Body.ThreadID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	body, err := cbornode.Decode(req.Body.Body, mh.SHA2_256, -1)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	rec, err := s.net.CreateRecord(ctx, id, body, net.WithThreadAuth(auth))
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

	id, err := thread.Cast(req.Body.ThreadID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	info, err := s.net.GetThread(ctx, id, net.WithThreadAuth(auth))
	if err != nil {
		return nil, err
	}
	logID, err := peer.IDFromBytes(req.Body.LogID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	rec, err := cbor.RecordFromProto(util.RecToServiceRec(req.Body.Record), info.Key.Service())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err = s.net.AddRecord(ctx, id, logID, rec, net.WithThreadAuth(auth)); err != nil {
		return nil, err
	}
	return &pb.AddRecordReply{}, nil
}

func (s *Service) GetRecord(ctx context.Context, req *pb.GetRecordRequest) (*pb.GetRecordReply, error) {
	log.Debugf("received get record request")

	id, err := thread.Cast(req.Body.ThreadID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	rid, err := cid.Cast(req.Body.RecordID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	rec, err := s.net.GetRecord(ctx, id, rid, net.WithThreadAuth(auth))
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

	opts := make([]net.SubOption, len(req.Body.ThreadIDs))
	for i, id := range req.Body.ThreadIDs {
		id, err := thread.Cast(id)
		if err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		opts[i] = net.WithSubFilter(id)
	}

	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return err
	}
	opts = append(opts, net.WithSubAuth(auth))

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

func getAuth(header *pb.Header, body proto.Message) (*thread.Auth, error) {
	if header.PubKey == nil {
		return nil, nil
	}
	pk, err := crypto.UnmarshalPublicKey(header.PubKey)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	auth := thread.NewAuthFromSignature(pk, header.Signature)
	msg, err := proto.Marshal(body)
	if err != nil {
		return nil, err
	}
	_, _, err = auth.Sign(msg) // Auth is already signed, but this will set the message for verification
	return auth, err
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
	return &pb.ThreadInfoReply{
		ThreadID:  info.ID.Bytes(),
		ThreadKey: info.Key.Bytes(),
		Logs:      logs,
	}, nil
}
