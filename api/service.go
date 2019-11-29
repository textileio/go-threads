package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/uuid"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-core/crypto/symmetric"
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
	store, err := s.getStore(req.GetStoreID())
	if err != nil {
		return nil, err
	}
	if err := store.Start(); err != nil {
		return nil, err
	}
	return &pb.StartReply{}, nil
}

func (s *service) StartFromAddress(ctx context.Context, req *pb.StartFromAddressRequest) (*pb.StartFromAddressReply, error) {
	var err error
	var store *es.Store
	var addr ma.Multiaddr
	var readKey, followKey *symmetric.Key
	if store, err = s.getStore(req.GetStoreID()); err != nil {
		return nil, err
	}
	if addr, err = ma.NewMultiaddr(req.GetAddress()); err != nil {
		return nil, err
	}
	if readKey, err = symmetric.NewKey([]byte(req.GetReadKey())); err != nil {
		return nil, err
	}
	if followKey, err = symmetric.NewKey([]byte(req.GetFollowKey())); err != nil {
		return nil, err
	}
	if err = store.StartFromAddr(addr, followKey, readKey); err != nil {
		return nil, err
	}
	return &pb.StartFromAddressReply{}, nil
}

// ModelCreate adds a new instance of a model to a store.
func (s *service) ModelCreate(ctx context.Context, req *pb.ModelCreateRequest) (*pb.ModelCreateReply, error) {
	log.Debugf("received model create request for model %s", req.ModelName)
	model, err := s.getModel(req.StoreID, req.ModelName)
	if err != nil {
		return nil, status.Error(codes.NotFound, "model not found")
	}
	return s.processCreateRequest(req, model.Create)
}

func (s *service) ModelSave(ctx context.Context, req *pb.ModelSaveRequest) (*pb.ModelSaveReply, error) {
	model, err := s.getModel(req.StoreID, req.ModelName)
	if err != nil {
		return nil, status.Error(codes.NotFound, "model not found")
	}
	return s.processSaveRequest(req, model.Save)
}

func (s *service) ModelDelete(ctx context.Context, req *pb.ModelDeleteRequest) (*pb.ModelDeleteReply, error) {
	model, err := s.getModel(req.StoreID, req.ModelName)
	if err != nil {
		return nil, status.Error(codes.NotFound, "model not found")
	}
	return s.processDeleteRequest(req, model.Delete)
}

func (s *service) ModelHas(ctx context.Context, req *pb.ModelHasRequest) (*pb.ModelHasReply, error) {
	model, err := s.getModel(req.StoreID, req.ModelName)
	if err != nil {
		return nil, status.Error(codes.NotFound, "model not found")
	}
	return s.processHasRequest(req, model.Has)
}

func (s *service) ModelFind(ctx context.Context, req *pb.ModelFindRequest) (*pb.ModelFindReply, error) {
	model, err := s.getModel(req.StoreID, req.ModelName)
	if err != nil {
		return nil, status.Error(codes.NotFound, "model not found")
	}
	return s.processFindRequest(req, model.FindJSON)
}

func (s *service) ModelFindByID(ctx context.Context, req *pb.ModelFindByIDRequest) (*pb.ModelFindByIDReply, error) {
	model, err := s.getModel(req.StoreID, req.ModelName)
	if err != nil {
		return nil, status.Error(codes.NotFound, "model not found")
	}
	return s.processFindByIDRequest(req, model.FindByID)
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

	model, err := s.getModel(storeID, modelName)
	if err != nil {
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
				innerReply, err := s.processFindByIDRequest(x.ModelFindByIDRequest, txn.FindByID)
				if err != nil {
					return err
				}
				option := &pb.ReadTransactionReply_ModelFindByIDReply{ModelFindByIDReply: innerReply}
				if err := stream.Send(&pb.ReadTransactionReply{Option: option}); err != nil {
					return err
				}
			case *pb.ReadTransactionRequest_ModelFindRequest:
				innerReply, err := s.processFindRequest(x.ModelFindRequest, txn.FindJSON)
				if err != nil {
					return err
				}
				option := &pb.ReadTransactionReply_ModelFindReply{ModelFindReply: innerReply}
				if err := stream.Send(&pb.ReadTransactionReply{Option: option}); err != nil {
					return err
				}
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
	firstReq, err := stream.Recv()
	if err != nil {
		return err
	}

	var storeID, modelName string
	switch x := firstReq.GetOption().(type) {
	case *pb.WriteTransactionRequest_StartTransactionRequest:
		storeID = x.StartTransactionRequest.GetStoreID()
		modelName = x.StartTransactionRequest.GetModelName()
	case nil:
		return fmt.Errorf("no WriteTransactionRequest type set")
	default:
		return fmt.Errorf("WriteTransactionRequest.Option has unexpected type %T", x)
	}

	model, err := s.getModel(storeID, modelName)
	if err != nil {
		return status.Error(codes.NotFound, "model not found")
	}

	err = model.WriteTxn(func(txn *es.Txn) error {
		for {
			req, err := stream.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			switch x := req.GetOption().(type) {
			case *pb.WriteTransactionRequest_ModelHasRequest:
				innerReply, err := s.processHasRequest(x.ModelHasRequest, txn.Has)
				if err != nil {
					return err
				}
				option := &pb.WriteTransactionReply_ModelHasReply{ModelHasReply: innerReply}
				if err := stream.Send(&pb.WriteTransactionReply{Option: option}); err != nil {
					return err
				}
			case *pb.WriteTransactionRequest_ModelFindByIDRequest:
				innerReply, err := s.processFindByIDRequest(x.ModelFindByIDRequest, txn.FindByID)
				if err != nil {
					return err
				}
				option := &pb.WriteTransactionReply_ModelFindByIDReply{ModelFindByIDReply: innerReply}
				if err := stream.Send(&pb.WriteTransactionReply{Option: option}); err != nil {
					return err
				}
			case *pb.WriteTransactionRequest_ModelFindRequest:
				innerReply, err := s.processFindRequest(x.ModelFindRequest, txn.FindJSON)
				if err != nil {
					return err
				}
				option := &pb.WriteTransactionReply_ModelFindReply{ModelFindReply: innerReply}
				if err := stream.Send(&pb.WriteTransactionReply{Option: option}); err != nil {
					return err
				}
			case *pb.WriteTransactionRequest_ModelCreateRequest:
				innerReply, err := s.processCreateRequest(x.ModelCreateRequest, txn.Create)
				if err != nil {
					return err
				}
				option := &pb.WriteTransactionReply_ModelCreateReply{ModelCreateReply: innerReply}
				if err := stream.Send(&pb.WriteTransactionReply{Option: option}); err != nil {
					return err
				}
			case *pb.WriteTransactionRequest_ModelSaveRequest:
				innerReply, err := s.processSaveRequest(x.ModelSaveRequest, txn.Save)
				if err != nil {
					return err
				}
				option := &pb.WriteTransactionReply_ModelSaveReply{ModelSaveReply: innerReply}
				if err := stream.Send(&pb.WriteTransactionReply{Option: option}); err != nil {
					return err
				}
			case *pb.WriteTransactionRequest_ModelDeleteRequest:
				innerReply, err := s.processDeleteRequest(x.ModelDeleteRequest, txn.Delete)
				if err != nil {
					return err
				}
				option := &pb.WriteTransactionReply_ModelDeleteReply{ModelDeleteReply: innerReply}
				if err := stream.Send(&pb.WriteTransactionReply{Option: option}); err != nil {
					return err
				}
			case nil:
				return fmt.Errorf("no WriteTransactionRequest type set")
			default:
				return fmt.Errorf("WriteTransactionRequest.Option has unexpected type %T", x)
			}
		}
	})

	// possibly nil
	return err
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

	for {
		select {
		case _ = <-server.Context().Done():
			return nil
		case _, ok := <-listener.Channel():
			if !ok {
				return nil
			}
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
	}
}

func (s *service) processCreateRequest(req *pb.ModelCreateRequest, createFunc func(...interface{}) error) (*pb.ModelCreateReply, error) {
	values := make([]interface{}, len(req.Values))
	for i, v := range req.Values {
		values[i] = &v
	}
	if err := createFunc(values...); err != nil {
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

func (s *service) processSaveRequest(req *pb.ModelSaveRequest, saveFunc func(...interface{}) error) (*pb.ModelSaveReply, error) {
	values := make([]interface{}, len(req.Values))
	for i, v := range req.Values {
		values[i] = &v
	}
	if err := saveFunc(values...); err != nil {
		return nil, err
	}
	return &pb.ModelSaveReply{}, nil
}

func (s *service) processDeleteRequest(req *pb.ModelDeleteRequest, deleteFunc func(...core.EntityID) error) (*pb.ModelDeleteReply, error) {
	entityIDs := make([]core.EntityID, len(req.GetEntityIDs()))
	for i, ID := range req.GetEntityIDs() {
		entityIDs[i] = core.EntityID(ID)
	}
	if err := deleteFunc(entityIDs...); err != nil {
		return nil, err
	}
	return &pb.ModelDeleteReply{}, nil
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

func (s *service) processFindByIDRequest(req *pb.ModelFindByIDRequest, findFunc func(id core.EntityID, v interface{}) error) (*pb.ModelFindByIDReply, error) {
	entityID := core.EntityID(req.EntityID)
	var result string
	if err := findFunc(entityID, &result); err != nil {
		return nil, err
	}
	return &pb.ModelFindByIDReply{Entity: result}, nil
}

func (s *service) processFindRequest(req *pb.ModelFindRequest, findFunc func(q es.JSONQuery) (ret []string, err error)) (*pb.ModelFindReply, error) {
	q := &es.JSONQuery{}
	if err := json.Unmarshal(req.GetQueryJSON(), q); err != nil {
		return nil, err
	}
	stringEntities, err := findFunc(*q)
	if err != nil {
		return nil, err
	}
	byteEntities := make([][]byte, len(stringEntities))
	for i, stringEntity := range stringEntities {
		byteEntities[i] = []byte(stringEntity)
	}
	return &pb.ModelFindReply{Entities: byteEntities}, nil
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

func (s *service) getModel(storeID string, modelName string) (*es.Model, error) {
	store, err := s.getStore(storeID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "store not found")
	}
	model := store.GetModel(modelName)
	if model == nil {
		return nil, status.Error(codes.NotFound, "model not found")
	}
	return model, nil
}
