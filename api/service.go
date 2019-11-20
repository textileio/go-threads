package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	ds "github.com/ipfs/go-datastore"
	core "github.com/textileio/go-textile-core/store"
	tserv "github.com/textileio/go-textile-core/threadservice"
	pb "github.com/textileio/go-textile-threads/api/pb"
	es "github.com/textileio/go-textile-threads/eventstore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type service struct {
	datastore ds.Datastore
	threads   tserv.Threadservice

	stores map[string]*es.Store
}

func (s *service) NewStore(ctx context.Context, req *pb.NewStoreRequest) (*pb.NewStoreReply, error) {
	log.Debugf("received new store request")

	store, err := es.NewStore(
		es.WithDatastore(s.datastore),
		es.WithThreadservice(s.threads),
		es.WithJsonMode(true),
		es.WithDebug(true))
	if err != nil {
		return nil, err
	}

	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	s.stores[id.String()] = store

	return &pb.NewStoreReply{
		ID: id.String(),
	}, nil
}

func (s *service) RegisterSchema(ctx context.Context, req *pb.RegisterSchemaRequest) (*pb.RegisterSchemaReply, error) {
	log.Debugf("received register schema request in store %s", req.StoreID)

	store, ok := s.stores[req.StoreID]
	if !ok {
		return nil, status.Error(codes.NotFound, "store not found")
	}

	_, err := store.RegisterSchema(req.Name, req.Schema)
	if err != nil {
		return nil, err
	}

	return &pb.RegisterSchemaReply{}, nil
}

func (s *service) ModelCreate(ctx context.Context, req *pb.ModelCreateRequest) (*pb.ModelCreateReply, error) {
	log.Debugf("received model create request for model %s", req.ModelName)

	store, ok := s.stores[req.StoreID]
	if !ok {
		return nil, status.Error(codes.NotFound, "store not found")
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
		EntityIDs: make([]string, len(values)),
	}
	for i, v := range values {
		id, err := getEntityID(v)
		if err != nil {
			return nil, err
		}
		reply.EntityIDs[i] = id.String()
	}

	return reply, nil
}

func getEntityID(t interface{}) (core.EntityID, error) {
	partial := &struct{ ID *string }{}
	if err := json.Unmarshal([]byte(*(t.(*string))), partial); err != nil {
		return "", fmt.Errorf("error when unmarshaling json instance: %v", err)
	}
	if partial.ID == nil {
		return "", fmt.Errorf("invalid instance: doesn't have an ID attribute")
	}
	if *partial.ID != "" && !core.IsValidEntityID(*partial.ID) {
		return "", fmt.Errorf("invalid instance: invalid ID value")
	}
	return core.EntityID(*partial.ID), nil
}
