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
	core "github.com/textileio/go-threads/core/service"
	"github.com/textileio/go-threads/core/thread"
	sym "github.com/textileio/go-threads/crypto/symmetric"
	pb "github.com/textileio/go-threads/service/api/pb"
	"github.com/textileio/go-threads/service/util"
	tutil "github.com/textileio/go-threads/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	log = logging.Logger("threadserviceapi")
)

// Service is a gRPC service for a thread service.
type Service struct {
	ts core.Service
}

// Config specifies server settings.
type Config struct {
	Debug bool
}

// NewService starts and returns a new service.
func NewService(ts core.Service, conf Config) (*Service, error) {
	var err error
	if conf.Debug {
		err = tutil.SetLogLevels(map[string]logging.LogLevel{
			"threadserviceapi": logging.LevelDebug,
		})
		if err != nil {
			return nil, err
		}
	}

	return &Service{ts: ts}, nil
}

// NewStore adds a new store into the manager.
func (s *Service) GetHostID(_ context.Context, _ *pb.GetHostIDRequest) (*pb.GetHostIDReply, error) {
	log.Debugf("received get host ID request")

	return &pb.GetHostIDReply{
		PeerID: marshalPeerID(s.ts.Host().ID()),
	}, nil
}

func (s *Service) CreateThread(ctx context.Context, req *pb.CreateThreadRequest) (*pb.ThreadInfoReply, error) {
	log.Debugf("received create thread request")

	threadID, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, err
	}
	opts, err := getKeyOptions(req.Keys)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	info, err := s.ts.CreateThread(ctx, threadID, opts...)
	if err != nil {
		return nil, err
	}
	return threadInfoToProto(info)
}

func (s *Service) AddThread(ctx context.Context, req *pb.AddThreadRequest) (*pb.ThreadInfoReply, error) {
	log.Debugf("received add thread request")

	addr, err := ma.NewMultiaddrBytes(req.Addr)
	if err != nil {
		return nil, err
	}
	opts, err := getKeyOptions(req.Keys)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	info, err := s.ts.AddThread(ctx, addr, opts...)
	if err != nil {
		return nil, err
	}
	return threadInfoToProto(info)
}

func (s *Service) GetThread(ctx context.Context, req *pb.GetThreadRequest) (*pb.ThreadInfoReply, error) {
	log.Debugf("received get thread request")

	threadID, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, err
	}
	info, err := s.ts.GetThread(ctx, threadID)
	if err != nil {
		return nil, err
	}
	return threadInfoToProto(info)
}

func (s *Service) PullThread(ctx context.Context, req *pb.PullThreadRequest) (*pb.PullThreadReply, error) {
	log.Debugf("received pull thread request")

	threadID, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, err
	}
	if err = s.ts.PullThread(ctx, threadID); err != nil {
		return nil, err
	}
	return &pb.PullThreadReply{}, nil
}

func (s *Service) DeleteThread(ctx context.Context, req *pb.DeleteThreadRequest) (*pb.DeleteThreadReply, error) {
	log.Debugf("received delete thread request")

	threadID, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, err
	}
	if err := s.ts.DeleteThread(ctx, threadID); err != nil {
		return nil, err
	}
	return &pb.DeleteThreadReply{}, nil
}

func (s *Service) AddFollower(ctx context.Context, req *pb.AddFollowerRequest) (*pb.AddFollowerReply, error) {
	log.Debugf("received add follower request")

	threadID, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, err
	}
	addr, err := ma.NewMultiaddrBytes(req.Addr)
	if err != nil {
		return nil, err
	}
	pid, err := s.ts.AddFollower(ctx, threadID, addr)
	if err != nil {
		return nil, err
	}
	return &pb.AddFollowerReply{
		PeerID: marshalPeerID(pid),
	}, nil
}

func (s *Service) CreateRecord(ctx context.Context, req *pb.CreateRecordRequest) (*pb.NewRecordReply, error) {
	log.Debugf("received create record request")

	threadID, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, err
	}
	body, err := cbornode.Decode(req.Body, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	rec, err := s.ts.CreateRecord(ctx, threadID, body)
	if err != nil {
		return nil, err
	}
	prec, err := cbor.RecordToProto(ctx, s.ts, rec.Value())
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

	threadID, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, err
	}
	info, err := s.ts.GetThread(ctx, threadID)
	if err != nil {
		return nil, err
	}
	logID, err := peer.IDFromBytes(req.LogID)
	if err != nil {
		return nil, err
	}
	rec, err := cbor.RecordFromProto(util.RecToServiceRec(req.Record), info.FollowKey)
	if err != nil {
		return nil, err
	}
	if err = s.ts.AddRecord(ctx, threadID, logID, rec); err != nil {
		return nil, err
	}
	return &pb.AddRecordReply{}, nil
}

func (s *Service) GetRecord(ctx context.Context, req *pb.GetRecordRequest) (*pb.GetRecordReply, error) {
	log.Debugf("received get record request")

	threadID, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, err
	}
	id, err := cid.Cast(req.RecordID)
	if err != nil {
		return nil, err
	}
	rec, err := s.ts.GetRecord(ctx, threadID, id)
	if err != nil {
		return nil, err
	}
	prec, err := cbor.RecordToProto(ctx, s.ts, rec)
	if err != nil {
		return nil, err
	}
	return &pb.GetRecordReply{
		Record: util.RecFromServiceRec(prec),
	}, nil
}

func (s *Service) Subscribe(req *pb.SubscribeRequest, server pb.API_SubscribeServer) error {
	log.Debugf("received subscribe request")

	opts := make([]core.SubOption, len(req.ThreadIDs))
	for i, id := range req.ThreadIDs {
		threadID, err := thread.Cast(id)
		if err != nil {
			return err
		}
		opts[i] = core.ThreadID(threadID)
	}

	sub, err := s.ts.Subscribe(server.Context(), opts...)
	if err != nil {
		return err
	}
	for rec := range sub {
		prec, err := cbor.RecordToProto(server.Context(), s.ts, rec.Value())
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

func getKeyOptions(keys *pb.ThreadKeys) (opts []core.KeyOption, err error) {
	if keys == nil {
		return
	}
	if keys.FollowKey != nil {
		fk, err := sym.FromBytes(keys.FollowKey)
		if err != nil {
			return nil, fmt.Errorf("invalid follow-key: %v", err)
		}
		opts = append(opts, core.FollowKey(fk))
	}
	if keys.ReadKey != nil {
		rk, err := sym.FromBytes(keys.ReadKey)
		if err != nil {
			return nil, fmt.Errorf("invalid read-key: %v", err)
		}
		opts = append(opts, core.ReadKey(rk))
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
		opts = append(opts, core.LogKey(lk))
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
		heads := make([][]byte, len(lg.Heads))
		for k, head := range lg.Heads {
			heads[k] = head.Bytes()
		}
		logs[i] = &pb.LogInfo{
			ID:      marshalPeerID(lg.ID),
			PubKey:  pk,
			PrivKey: sk,
			Addrs:   addrs,
			Heads:   heads,
		}
	}
	var rk []byte
	if info.ReadKey != nil {
		rk = info.ReadKey.Bytes()
	}
	return &pb.ThreadInfoReply{
		ID:        info.ID.Bytes(),
		Logs:      logs,
		ReadKey:   rk,
		FollowKey: info.FollowKey.Bytes(),
	}, nil
}
