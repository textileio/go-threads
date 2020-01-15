package api

import (
	"context"

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
		PeerID: s.s.Host().ID().String(),
	}, nil
}

func (s *service) AddThread(ctx context.Context, req *pb.AddThreadRequest) (*pb.AddThreadReply, error) {
	log.Debugf("received add thread request")

	addr, err := ma.NewMultiaddr(req.Addr)
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
	logIDs := make([]string, len(info.Logs))
	for i, lid := range info.Logs {
		logIDs[i] = lid.String()
	}
	return &pb.AddThreadReply{
		ThreadID: info.ID.String(),
		LogIDs:   logIDs,
	}, nil
}

func (s *service) PullThread(ctx context.Context, req *pb.PullThreadRequest) (*pb.PullThreadReply, error) {
	log.Debugf("received pull thread request")

	threadID, err := thread.Decode(req.ThreadID)
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

	threadID, err := thread.Decode(req.ThreadID)
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

	threadID, err := thread.Decode(req.ThreadID)
	if err != nil {
		return nil, err
	}
	peerID, err := peer.Decode(req.PeerID)
	if err != nil {
		return nil, err
	}
	if err := s.s.AddFollower(ctx, threadID, peerID); err != nil {
		return nil, err
	}
	return &pb.AddFollowerReply{}, nil
}

func (s *service) AddRecord(ctx context.Context, req *pb.AddRecordRequest) (*pb.NewRecordReply, error) {
	log.Debugf("received add record request")

	threadID, err := thread.Decode(req.ThreadID)
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
		ThreadID: rec.ThreadID().String(),
		LogID:    rec.LogID().String(),
		RecordID: rec.Value().Cid().String(),
	}, nil
}

func (s *service) GetRecord(ctx context.Context, req *pb.GetRecordRequest) (*pb.GetRecordReply, error) {
	log.Debugf("received get record request")

	threadID, err := thread.Decode(req.ThreadID)
	if err != nil {
		return nil, err
	}
	id, err := cid.Decode(req.RecordID)
	if err != nil {
		return nil, err
	}
	rec, err := s.s.GetRecord(ctx, threadID, id)
	if err != nil {
		return nil, err
	}
	return &pb.GetRecordReply{
		Record: rec.RawData(),
	}, nil
}

func (s *service) Subscribe(req *pb.SubscribeRequest, server pb.API_SubscribeServer) error {
	log.Debugf("received subscribe request")

	opts := make([]core.SubOption, len(req.ThreadIDs))
	for i, id := range req.ThreadIDs {
		threadID, err := thread.Decode(id)
		if err != nil {
			return err
		}
		opts[i] = core.ThreadID(threadID)
	}
	sub := s.s.Subscribe(opts...)
	defer sub.Discard()

	for {
		select {
		case <-server.Context().Done():
			return nil
		case rec, ok := <-sub.Channel():
			if !ok {
				return nil
			}
			if err := server.Send(&pb.NewRecordReply{
				ThreadID: rec.ThreadID().String(),
				LogID:    rec.LogID().String(),
				RecordID: rec.Value().Cid().String(),
			}); err != nil {
				return err
			}
		}
	}
}
