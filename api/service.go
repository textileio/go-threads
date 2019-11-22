package api

import (
	"context"

	"github.com/google/uuid"
	core "github.com/textileio/go-textile-core/store"
	pb "github.com/textileio/go-textile-threads/api/pb"
	es "github.com/textileio/go-textile-threads/eventstore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// service is a gRPC service for a store manager.
type service struct {
	manager *es.Manager
}

// NewStore adds a new store into the manager.
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

// RegisterSchema registers a JSON schema with a store.
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

// ModelCreate adds a new instance of a model to a store.
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

// Listen returns a stream of entities, trigged by a local or remote state change.
func (s *service) Listen(req *pb.ListenRequest, server pb.API_ListenServer) error {
	log.Debugf("received listen request for entity %s", req.EntityID)

	store, err := s.getStore(req.StoreID)
	if err != nil {
		return err
	}

	model := store.GetModel(req.ModelName)
	if model == nil {
		return status.Error(codes.NotFound, "model not found")
	}

	listener := store.StateChangeListen()
	defer listener.Discard()
	for range listener.Channel() {
		err := model.ReadTxn(func(txn *es.Txn) error {
			var res string
			if err := txn.FindByID(core.EntityID(req.EntityID), &res); err != nil {
				return err
			}
			return server.Send(&pb.ListenReply{
				Entity: res,
			})
		})
		if err != nil {
			return err
		}
	}
	return nil
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
