package api

import (
	"context"

	"github.com/textileio/go-threads/core/thread"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/crypto/symmetric"

	core "github.com/textileio/go-threads/core/service"
	pb "github.com/textileio/go-threads/service/api/pb"
)

// service is a gRPC service for a thread service.
type service struct {
	s core.Service
}

// NewStore adds a new store into the manager.
func (s *service) GetHostID(ctx context.Context, req *pb.GetHostIDRequest) (*pb.GetHostIDReply, error) {
	log.Debugf("received get host ID request")

	return &pb.GetHostIDReply{
		ID: s.s.Host().ID().String(),
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
	if _, err = s.s.AddThread(ctx, addr, opts...); err != nil {
		return nil, err
	}
	return &pb.AddThreadReply{}, nil
}

func (s *service) PullThread(ctx context.Context, req *pb.PullThreadRequest) (*pb.PullThreadReply, error) {
	log.Debugf("received pull thread request")

	id, err := thread.Decode(req.ID)
	if err != nil {
		return nil, err
	}
	if err = s.s.PullThread(ctx, id); err != nil {
		return nil, err
	}
	return &pb.PullThreadReply{}, nil
}

func (s *service) DeleteThread(ctx context.Context, req *pb.DeleteThreadRequest) (*pb.DeleteThreadReply, error) {
	panic("implement me")
}

func (s *service) AddFollower(ctx context.Context, req *pb.AddFollowerRequest) (*pb.AddFollowerReply, error) {
	panic("implement me")
}

func (s *service) AddRecord(ctx context.Context, req *pb.AddRecordRequest) (*pb.AddRecordReply, error) {
	panic("implement me")
}

func (s *service) GetRecord(ctx context.Context, req *pb.GetRecordRequest) (*pb.GetRecordReply, error) {
	panic("implement me")
}

func (s *service) Subscribe(req *pb.SubscribeRequest, server pb.API_SubscribeServer) error {
	panic("implement me")
}
