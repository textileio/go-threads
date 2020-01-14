package api

import (
	"context"

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

func (s *service) AddThread(context.Context, *pb.AddThreadRequest) (*pb.AddThreadReply, error) {
	panic("implement me")
}

func (s *service) PullThread(context.Context, *pb.PullThreadRequest) (*pb.PullThreadReply, error) {
	panic("implement me")
}

func (s *service) DeleteThread(context.Context, *pb.DeleteThreadRequest) (*pb.DeleteThreadReply, error) {
	panic("implement me")
}

func (s *service) AddFollower(context.Context, *pb.AddFollowerRequest) (*pb.AddFollowerReply, error) {
	panic("implement me")
}

func (s *service) AddRecord(context.Context, *pb.AddRecordRequest) (*pb.AddRecordReply, error) {
	panic("implement me")
}

func (s *service) GetRecord(context.Context, *pb.GetRecordRequest) (*pb.GetRecordReply, error) {
	panic("implement me")
}

func (s *service) Subscribe(*pb.SubscribeRequest, pb.API_SubscribeServer) error {
	panic("implement me")
}
