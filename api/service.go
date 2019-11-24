package api

import (
	"context"
	"fmt"
	"io"

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

func (s *service) Start(ctx context.Context, req *pb.StartRequest) (*pb.StartReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Start not implemented")
}

func (s *service) StartFromAddress(ctx context.Context, req *pb.StartFromAddressRequest) (*pb.StartFromAddressReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StartFromAddress not implemented")
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

func (s *service) ModelSave(ctx context.Context, req *pb.ModelSaveRequest) (*pb.ModelSaveReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ModelSave not implemented")
}

func (s *service) ModelDelete(ctx context.Context, req *pb.ModelDeleteRequest) (*pb.ModelDeleteReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ModelDelete not implemented")
}

func (s *service) ModelHas(ctx context.Context, req *pb.ModelHasRequest) (*pb.ModelHasReply, error) {
	store, err := s.getStore(req.StoreID)
	if err != nil {
		return nil, err
	}

	model := store.GetModel(req.ModelName)
	if model == nil {
		return nil, status.Error(codes.NotFound, "model not found")
	}

	return s.processHasRequest(req, model.Has)
}

func (s *service) ModelFind(ctx context.Context, req *pb.ModelFindRequest) (*pb.ModelFindReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ModelFind not implemented")
}

func (s *service) ModelFindByID(ctx context.Context, req *pb.ModelFindByIDRequest) (*pb.ModelFindByIDReply, error) {
	store, err := s.getStore(req.StoreID)
	if err != nil {
		return nil, err
	}

	model := store.GetModel(req.ModelName)
	if model == nil {
		return nil, status.Error(codes.NotFound, "model not found")
	}

	type Foo struct{}

	return s.processFindByIDRequest(req, model.FindByID, &Foo{})
}

func (s *service) ReadTransaction(stream pb.API_ReadTransactionServer) error {
	firstReq, err := stream.Recv()
	if err != nil {
		return err
	}

	var storeID, modelName string
	switch x := firstReq.GetOption().(type) {
	case *pb.ReadTransactionRequest_StartTransactionRequest:
		storeID = x.StartTransactionRequest.GetStoreID()
		modelName = x.StartTransactionRequest.GetModelName()
	case nil:
		return fmt.Errorf("no ReadTransactionRequest type set")
	default:
		return fmt.Errorf("ReadTransactionRequest.Option has unexpected type %T", x)
	}

	store, err := s.getStore(storeID)
	if err != nil {
		return err
	}

	model := store.GetModel(modelName)
	if model == nil {
		return status.Error(codes.NotFound, "model not found")
	}

	err = model.ReadTxn(func(txn *es.Txn) error {
		for {
			req, err := stream.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			switch x := req.GetOption().(type) {
			case *pb.ReadTransactionRequest_ModelHasRequest:
				innerReply, err := s.processHasRequest(x.ModelHasRequest, txn.Has)
				if err != nil {
					return err
				}
				option := &pb.ReadTransactionReply_ModelHasReply{ModelHasReply: innerReply}
				if err := stream.Send(&pb.ReadTransactionReply{Option: option}); err != nil {
					return err
				}
			case *pb.ReadTransactionRequest_ModelFindByIDRequest:
				// find by id
			case *pb.ReadTransactionRequest_ModelFindRequest:
				// find
			case nil:
				return fmt.Errorf("no ReadTransactionRequest type set")
			default:
				return fmt.Errorf("ReadTransactionRequest.Option has unexpected type %T", x)
			}
		}
	})

	// possibly nil
	return err
}

func (s *service) WriteTransaction(stream pb.API_WriteTransactionServer) error {
	return status.Errorf(codes.Unimplemented, "method WriteTransaction not implemented")
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

func (s *service) processHasRequest(req *pb.ModelHasRequest, hasFunc func(...core.EntityID) (bool, error)) (*pb.ModelHasReply, error) {
	entityIDs := make([]core.EntityID, len(req.GetEntityIDs()))
	for i, ID := range req.GetEntityIDs() {
		entityIDs[i] = core.EntityID(ID)
	}
	exists, err := hasFunc(entityIDs...)
	if err != nil {
		return nil, err
	}
	return &pb.ModelHasReply{Exists: exists}, nil
}

func (s *service) processFindByIDRequest(req *pb.ModelFindByIDRequest, findFunc func(id core.EntityID, v interface{}) error, dest interface{}) (*pb.ModelFindByIDReply, error) {
	entityID := core.EntityID(req.EntityID)
	if err := findFunc(entityID, dest); err != nil {
		return nil, err
	}
	// marshall dest into json string
	return &pb.ModelFindByIDReply{Entity: "json string"}, nil
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
