package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/uuid"
	ma "github.com/multiformats/go-multiaddr"
	pb "github.com/textileio/go-threads/api/pb"
	corestore "github.com/textileio/go-threads/core/store"
	"github.com/textileio/go-threads/crypto/symmetric"
	"github.com/textileio/go-threads/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// service is a gRPC service for a store manager.
type service struct {
	manager *store.Manager
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

	st, err := s.getStore(req.StoreID)
	if err != nil {
		return nil, err
	}
	if _, err = st.RegisterSchema(req.Name, req.Schema, []string{}); err != nil {
		return nil, err
	}

	return &pb.RegisterSchemaReply{}, nil
}

func (s *service) Start(ctx context.Context, req *pb.StartRequest) (*pb.StartReply, error) {
	st, err := s.getStore(req.GetStoreID())
	if err != nil {
		return nil, err
	}
	if err := st.Start(); err != nil {
		return nil, err
	}
	return &pb.StartReply{}, nil
}

func (s *service) GetStoreLink(ctx context.Context, req *pb.GetStoreLinkRequest) (*pb.GetStoreLinkReply, error) {
	var err error
	var st *store.Store
	if st, err = s.getStore(req.GetStoreID()); err != nil {
		return nil, err
	}
	tid, _, err := st.ThreadID()
	if err != nil {
		return nil, err
	}
	tinfo, err := st.Service().Store().ThreadInfo(tid)
	if err != nil {
		return nil, err
	}
	host := st.Service().Host()
	id, _ := ma.NewComponent("p2p", host.ID().String())
	thread, _ := ma.NewComponent("thread", tid.String())
	addrs := host.Addrs()
	res := make([]string, len(addrs))
	for i := range addrs {
		res[i] = addrs[i].Encapsulate(id).Encapsulate(thread).String()
	}
	reply := &pb.GetStoreLinkReply{
		Addresses: res,
		FollowKey: tinfo.FollowKey.Bytes(),
		ReadKey:   tinfo.ReadKey.Bytes(),
	}
	return reply, nil
}

func (s *service) StartFromAddress(ctx context.Context, req *pb.StartFromAddressRequest) (*pb.StartFromAddressReply, error) {
	var err error
	var st *store.Store
	var addr ma.Multiaddr
	var readKey, followKey *symmetric.Key
	if st, err = s.getStore(req.GetStoreID()); err != nil {
		return nil, err
	}
	if addr, err = ma.NewMultiaddr(req.GetAddress()); err != nil {
		return nil, err
	}
	if readKey, err = symmetric.NewKey(req.GetReadKey()); err != nil {
		return nil, err
	}
	if followKey, err = symmetric.NewKey(req.GetFollowKey()); err != nil {
		return nil, err
	}
	if err = st.StartFromAddr(addr, followKey, readKey); err != nil {
		return nil, err
	}
	return &pb.StartFromAddressReply{}, nil
}

// ModelCreate adds a new instance of a model to a store.
func (s *service) ModelCreate(ctx context.Context, req *pb.ModelCreateRequest) (*pb.ModelCreateReply, error) {
	log.Debugf("received model create request for model %s", req.ModelName)
	model, err := s.getModel(req.StoreID, req.ModelName)
	if err != nil {
		return nil, err
	}
	return s.processCreateRequest(req, model.Create)
}

func (s *service) ModelSave(ctx context.Context, req *pb.ModelSaveRequest) (*pb.ModelSaveReply, error) {
	model, err := s.getModel(req.StoreID, req.ModelName)
	if err != nil {
		return nil, err
	}
	return s.processSaveRequest(req, model.Save)
}

func (s *service) ModelDelete(ctx context.Context, req *pb.ModelDeleteRequest) (*pb.ModelDeleteReply, error) {
	model, err := s.getModel(req.StoreID, req.ModelName)
	if err != nil {
		return nil, err
	}
	return s.processDeleteRequest(req, model.Delete)
}

func (s *service) ModelHas(ctx context.Context, req *pb.ModelHasRequest) (*pb.ModelHasReply, error) {
	model, err := s.getModel(req.StoreID, req.ModelName)
	if err != nil {
		return nil, err
	}
	return s.processHasRequest(req, model.Has)
}

func (s *service) ModelFind(ctx context.Context, req *pb.ModelFindRequest) (*pb.ModelFindReply, error) {
	model, err := s.getModel(req.StoreID, req.ModelName)
	if err != nil {
		return nil, err
	}
	return s.processFindRequest(req, model.FindJSON)
}

func (s *service) ModelFindByID(ctx context.Context, req *pb.ModelFindByIDRequest) (*pb.ModelFindByIDReply, error) {
	model, err := s.getModel(req.StoreID, req.ModelName)
	if err != nil {
		return nil, err
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
		return err
	}

	return model.ReadTxn(func(txn *store.Txn) error {
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
		return err
	}

	return model.WriteTxn(func(txn *store.Txn) error {
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
}

// Listen returns a stream of entities, trigged by a local or remote state change.
func (s *service) Listen(req *pb.ListenRequest, server pb.API_ListenServer) error {
	st, err := s.getStore(req.StoreID)
	if err != nil {
		return err
	}

	options := make([]store.ListenOption, len(req.GetFilters()))
	for i, filter := range req.GetFilters() {
		var listenActionType store.ListenActionType
		switch filter.GetAction() {
		case pb.ListenRequest_Filter_ALL:
			listenActionType = store.ListenAll
		case pb.ListenRequest_Filter_CREATE:
			listenActionType = store.ListenCreate
		case pb.ListenRequest_Filter_DELETE:
			listenActionType = store.ListenDelete
		case pb.ListenRequest_Filter_SAVE:
			listenActionType = store.ListenSave
		default:
			return status.Errorf(codes.InvalidArgument, "invalid filter action %v", filter.GetAction())
		}
		options[i] = store.ListenOption{
			Type:  listenActionType,
			Model: filter.GetModelName(),
			ID:    corestore.EntityID(filter.EntityID),
		}
	}

	l, err := st.Listen(options...)
	if err != nil {
		return err
	}
	defer l.Close()

	for {
		err = nil
		select {
		case <-server.Context().Done():
			return nil
		case action, ok := <-l.Channel():
			if !ok {
				return nil
			}
			var replyAction pb.ListenReply_Action
			var entity []byte
			switch action.Type {
			case store.ActionCreate:
				replyAction = pb.ListenReply_CREATE
				entity, err = s.entityForAction(st, action)
			case store.ActionDelete:
				replyAction = pb.ListenReply_DELETE
			case store.ActionSave:
				replyAction = pb.ListenReply_SAVE
				entity, err = s.entityForAction(st, action)
			default:
				err = status.Errorf(codes.Internal, "unknown action type %v", action.Type)
			}
			if err != nil {
				return err
			}
			reply := &pb.ListenReply{
				ModelName: action.Model,
				EntityID:  action.ID.String(),
				Action:    replyAction,
				Entity:    entity,
			}
			err := server.Send(reply)
			if err != nil {
				return err
			}
		}
	}
}

func (s *service) entityForAction(store *store.Store, action store.Action) ([]byte, error) {
	model := store.GetModel(action.Model)
	if model == nil {
		return nil, status.Error(codes.NotFound, "model not found")
	}
	var res string
	if err := model.FindByID(action.ID, &res); err != nil {
		return nil, err
	}
	return []byte(res), nil
}

func (s *service) processCreateRequest(req *pb.ModelCreateRequest, createFunc func(...interface{}) error) (*pb.ModelCreateReply, error) {
	values := make([]interface{}, len(req.Values))
	for i, v := range req.Values {
		s := v
		values[i] = &s
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
		s := v
		values[i] = &s
	}
	if err := saveFunc(values...); err != nil {
		return nil, err
	}
	return &pb.ModelSaveReply{}, nil
}

func (s *service) processDeleteRequest(req *pb.ModelDeleteRequest, deleteFunc func(...corestore.EntityID) error) (*pb.ModelDeleteReply, error) {
	entityIDs := make([]corestore.EntityID, len(req.GetEntityIDs()))
	for i, ID := range req.GetEntityIDs() {
		entityIDs[i] = corestore.EntityID(ID)
	}
	if err := deleteFunc(entityIDs...); err != nil {
		return nil, err
	}
	return &pb.ModelDeleteReply{}, nil
}

func (s *service) processHasRequest(req *pb.ModelHasRequest, hasFunc func(...corestore.EntityID) (bool, error)) (*pb.ModelHasReply, error) {
	entityIDs := make([]corestore.EntityID, len(req.GetEntityIDs()))
	for i, ID := range req.GetEntityIDs() {
		entityIDs[i] = corestore.EntityID(ID)
	}
	exists, err := hasFunc(entityIDs...)
	if err != nil {
		return nil, err
	}
	return &pb.ModelHasReply{Exists: exists}, nil
}

func (s *service) processFindByIDRequest(req *pb.ModelFindByIDRequest, findFunc func(id corestore.EntityID, v interface{}) error) (*pb.ModelFindByIDReply, error) {
	entityID := corestore.EntityID(req.EntityID)
	var result string
	if err := findFunc(entityID, &result); err != nil {
		return nil, err
	}
	return &pb.ModelFindByIDReply{Entity: result}, nil
}

func (s *service) processFindRequest(req *pb.ModelFindRequest, findFunc func(q *store.JSONQuery) (ret []string, err error)) (*pb.ModelFindReply, error) {
	q := &store.JSONQuery{}
	if err := json.Unmarshal(req.GetQueryJSON(), q); err != nil {
		return nil, err
	}
	stringEntities, err := findFunc(q)
	if err != nil {
		return nil, err
	}
	byteEntities := make([][]byte, len(stringEntities))
	for i, stringEntity := range stringEntities {
		byteEntities[i] = []byte(stringEntity)
	}
	return &pb.ModelFindReply{Entities: byteEntities}, nil
}

func (s *service) getStore(idStr string) (*store.Store, error) {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, err
	}
	st := s.manager.GetStore(id)
	if st == nil {
		return nil, status.Error(codes.NotFound, "store not found")
	}
	return st, nil
}

func (s *service) getModel(storeID string, modelName string) (*store.Model, error) {
	st, err := s.getStore(storeID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "store not found")
	}
	model := st.GetModel(modelName)
	if model == nil {
		return nil, status.Error(codes.NotFound, "model not found")
	}
	return model, nil
}
