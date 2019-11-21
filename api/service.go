package api

import (
	"context"

	"github.com/google/uuid"
	pb "github.com/textileio/go-textile-threads/api/pb"
	es "github.com/textileio/go-textile-threads/eventstore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type service struct {
	manager *es.Manager
}

func (s *service) NewStore(ctx context.Context, req *pb.NewStoreRequest) (*pb.NewStoreReply, error) {
	log.Debugf("received new store request")

	id, _, err := s.manager.NewStore()
	if err != nil {
		return nil, err
	}

	return &pb.NewStoreReply{
		ID: id.String(),
	}, nil
}

func (s *service) RegisterSchema(ctx context.Context, req *pb.RegisterSchemaRequest) (*pb.RegisterSchemaReply, error) {
	log.Debugf("received register schema request in store %s", req.StoreID)

	store, err := s.getStore(req.StoreID)
	if err != nil {
		return nil, err
	}
	if _, err = store.RegisterSchema(req.Name, req.Schema); err != nil {
		return nil, err
	}

	return &pb.RegisterSchemaReply{}, nil
}

func (s *service) ModelCreate(ctx context.Context, req *pb.ModelCreateRequest) (*pb.ModelCreateReply, error) {
	log.Debugf("received model create request for model %s", req.ModelName)

	store, err := s.getStore(req.StoreID)
	if err != nil {
		return nil, err
	}

	model := store.GetModel(req.ModelName)
	if model == nil {
		return nil, status.Error(codes.NotFound, "model not found")
	}

	values := make([]interface{}, len(req.Values))
	for i, v := range req.Values {
		values[i] = &v
	}
	if err := model.Create(values...); err != nil {
		return nil, err
	}

	reply := &pb.ModelCreateReply{
		Entities: make([]string, len(values)),
	}
	for i, v := range values {
		reply.Entities[i] = *(v.(*string))
	}

	return reply, nil
}

func (s *service) getStore(idStr string) (*es.Store, error) {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, err
	}
	store := s.manager.GetStore(id)
	if store == nil {
		return nil, status.Error(codes.NotFound, "store not found")
	}
	return store, nil
}
