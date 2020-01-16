package api

import (
	"context"

	"github.com/libp2p/go-libp2p-core/crypto"

	"github.com/ipfs/go-cid"
	cbornode "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	core "github.com/textileio/go-threads/core/service"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/crypto/symmetric"
	pb "github.com/textileio/go-threads/service/api/pb"
)

// service is a gRPC service for a thread service.
type service struct {
	s core.Service
}

// NewStore adds a new store into the manager.
func (s *service) GetHostID(context.Context, *pb.GetHostIDRequest) (*pb.GetHostIDReply, error) {
	log.Debugf("received get host ID request")

	return &pb.GetHostIDReply{
		PeerID: marshalPeerID(s.s.Host().ID()),
	}, nil
}

func (s *service) CreateThread(ctx context.Context, req *pb.CreateThreadRequest) (*pb.ThreadInfoReply, error) {
	log.Debugf("received create thread request")

	threadID, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, err
	}
	info, err := s.s.CreateThread(ctx, threadID)
	if err != nil {
		return nil, err
	}
	return pbThreadFromInfo(info)
}

func (s *service) AddThread(ctx context.Context, req *pb.AddThreadRequest) (*pb.ThreadInfoReply, error) {
	log.Debugf("received add thread request")

	addr, err := ma.NewMultiaddrBytes(req.Addr)
	if err != nil {
		return nil, err
	}
	var opts []core.AddOption
	if req.ReadKey != nil {
		rk, err := symmetric.NewKey(req.ReadKey)
		if err != nil {
			return nil, err
		}
		opts = append(opts, core.ReadKey(rk))
	}
	if req.FollowKey != nil {
		fk, err := symmetric.NewKey(req.FollowKey)
		if err != nil {
			return nil, err
		}
		opts = append(opts, core.FollowKey(fk))
	}
	info, err := s.s.AddThread(ctx, addr, opts...)
	if err != nil {
		return nil, err
	}
	return pbThreadFromInfo(info)
}

func (s *service) GetThread(ctx context.Context, req *pb.GetThreadRequest) (*pb.ThreadInfoReply, error) {
	log.Debugf("received get thread request")

	threadID, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, err
	}
	info, err := s.s.GetThread(ctx, threadID)
	if err != nil {
		return nil, err
	}
	return pbThreadFromInfo(info)
}

func (s *service) PullThread(ctx context.Context, req *pb.PullThreadRequest) (*pb.PullThreadReply, error) {
	log.Debugf("received pull thread request")

	threadID, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, err
	}
	if err = s.s.PullThread(ctx, threadID); err != nil {
		return nil, err
	}
	return &pb.PullThreadReply{}, nil
}

func (s *service) DeleteThread(ctx context.Context, req *pb.DeleteThreadRequest) (*pb.DeleteThreadReply, error) {
	log.Debugf("received delete thread request")

	threadID, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, err
	}
	if err := s.s.DeleteThread(ctx, threadID); err != nil {
		return nil, err
	}
	return &pb.DeleteThreadReply{}, nil
}

func (s *service) AddFollower(ctx context.Context, req *pb.AddFollowerRequest) (*pb.AddFollowerReply, error) {
	log.Debugf("received add follower request")

	threadID, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, err
	}
	addr, err := ma.NewMultiaddrBytes(req.Addr)
	if err != nil {
		return nil, err
	}
	pid, err := s.s.AddFollower(ctx, threadID, addr)
	if err != nil {
		return nil, err
	}
	return &pb.AddFollowerReply{
		PeerID: marshalPeerID(pid),
	}, nil
}

func (s *service) AddRecord(ctx context.Context, req *pb.AddRecordRequest) (*pb.NewRecordReply, error) {
	log.Debugf("received add record request")

	threadID, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, err
	}
	body, err := cbornode.Decode(req.Body, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	rec, err := s.s.AddRecord(ctx, threadID, body)
	if err != nil {
		return nil, err
	}
	return &pb.NewRecordReply{
		ThreadID: rec.ThreadID().Bytes(),
		LogID:    marshalPeerID(rec.LogID()),
		Record:   pbRecordFromRecord(rec.Value()),
	}, nil
}

func (s *service) GetRecord(ctx context.Context, req *pb.GetRecordRequest) (*pb.GetRecordReply, error) {
	log.Debugf("received get record request")

	threadID, err := thread.Cast(req.ThreadID)
	if err != nil {
		return nil, err
	}
	id, err := cid.Cast(req.RecordID)
	if err != nil {
		return nil, err
	}
	rec, err := s.s.GetRecord(ctx, threadID, id)
	if err != nil {
		return nil, err
	}
	return &pb.GetRecordReply{
		Record: pbRecordFromRecord(rec),
	}, nil
}

func (s *service) Subscribe(req *pb.SubscribeRequest, server pb.API_SubscribeServer) error {
	log.Debugf("received subscribe request")

	opts := make([]core.SubOption, len(req.ThreadIDs))
	for i, id := range req.ThreadIDs {
		threadID, err := thread.Cast(id)
		if err != nil {
			return err
		}
		opts[i] = core.ThreadID(threadID)
	}

	sub, err := s.s.Subscribe(server.Context(), opts...)
	if err != nil {
		return err
	}
	for rec := range sub {
		if err := server.Send(&pb.NewRecordReply{
			ThreadID: rec.ThreadID().Bytes(),
			LogID:    marshalPeerID(rec.LogID()),
			Record:   pbRecordFromRecord(rec.Value()),
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

func pbThreadFromInfo(info thread.Info) (*pb.ThreadInfoReply, error) {
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

func pbRecordFromRecord(rec core.Record) *pb.Record {
	return &pb.Record{
		Node:    rec.RawData(),
		BlockID: rec.BlockID().Bytes(),
		Sig:     rec.Sig(),
		PrevID:  rec.PrevID().Bytes(),
	}
}
