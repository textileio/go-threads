package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/alecthomas/jsonschema"
	"github.com/golang/protobuf/proto"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/crypto"
	ma "github.com/multiformats/go-multiaddr"
	pb "github.com/textileio/go-threads/api/pb"
	core "github.com/textileio/go-threads/core/db"
	lstore "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	"github.com/textileio/go-threads/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	log = logging.Logger("threadsapi")
)

// Service is a gRPC service for a DB manager.
type Service struct {
	manager *db.Manager
}

// Config specifies server settings.
type Config struct {
	RepoPath string
	Debug    bool
}

// NewService starts and returns a new service with the given threadservice.
// The threadnet is *not* managed by the server.
func NewService(network core.Net, conf Config) (*Service, error) {
	var err error
	if conf.Debug {
		err = util.SetLogLevels(map[string]logging.LogLevel{
			"threadsapi": logging.LevelDebug,
		})
		if err != nil {
			return nil, err
		}
	}

	manager, err := db.NewManager(network, db.WithRepoPath(conf.RepoPath), db.WithDebug(conf.Debug))
	if err != nil {
		return nil, err
	}
	return &Service{manager: manager}, nil
}

// Close the service and the db manager.
func (s *Service) Close() error {
	return s.manager.Close()
}

// NewDB adds a new db into the manager.
func (s *Service) NewDB(ctx context.Context, req *pb.NewDBRequest) (*pb.NewDBReply, error) {
	log.Debugf("received new db request")

	id, err := thread.Cast(req.Body.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	if _, err = s.manager.NewDB(ctx, id, db.WithNewManagedDBAuth(auth)); err != nil {
		return nil, err
	}
	return &pb.NewDBReply{}, nil
}

// NewDBFromAddr adds a new db into the manager from an existing address.
func (s *Service) NewDBFromAddr(ctx context.Context, req *pb.NewDBFromAddrRequest) (*pb.NewDBReply, error) {
	log.Debugf("received new db from address request")

	addr, err := ma.NewMultiaddrBytes(req.Body.Addr)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	key, err := thread.KeyFromBytes(req.Body.Key)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	collections := make([]db.CollectionConfig, len(req.Body.Collections))
	for i, c := range req.Body.Collections {
		cc, err := collectionConfigFromPb(c)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		collections[i] = cc
	}
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	if _, err = s.manager.NewDBFromAddr(ctx, addr, key, db.WithNewManagedDBAuth(auth), db.WithNewManagedDBCollections(collections...)); err != nil {
		return nil, err
	}
	return &pb.NewDBReply{}, nil
}

func collectionConfigFromPb(pbc *pb.CollectionConfig) (db.CollectionConfig, error) {
	indexes := make([]db.IndexConfig, len(pbc.Indexes))
	for i, index := range pbc.Indexes {
		indexes[i] = db.IndexConfig{
			Path:   index.Path,
			Unique: index.Unique,
		}
	}
	schema := &jsonschema.Schema{}
	if err := json.Unmarshal(pbc.GetSchema(), schema); err != nil {
		return db.CollectionConfig{}, err
	}
	return db.CollectionConfig{
		Name:    pbc.Name,
		Schema:  schema,
		Indexes: indexes,
	}, nil
}

// GetDBInfo returns db addresses and keys.
func (s *Service) GetDBInfo(ctx context.Context, req *pb.GetDBInfoRequest) (*pb.GetDBInfoReply, error) {
	id, err := thread.Cast(req.Body.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	tinfo, err := s.manager.Net().GetThread(ctx, id, net.WithThreadAuth(auth))
	if err != nil {
		return nil, err
	}
	host := s.manager.Net().Host()
	peerID, _ := ma.NewComponent("p2p", host.ID().String())
	threadID, _ := ma.NewComponent("thread", tinfo.ID.String())
	addrs := host.Addrs()
	res := make([][]byte, len(addrs))
	for i := range addrs {
		res[i] = addrs[i].Encapsulate(peerID).Encapsulate(threadID).Bytes()
	}
	reply := &pb.GetDBInfoReply{
		Addrs: res,
		Key:   tinfo.Key.Bytes(),
	}
	return reply, nil
}

// NewCollection registers a JSON schema with a db.
func (s *Service) NewCollection(ctx context.Context, req *pb.NewCollectionRequest) (*pb.NewCollectionReply, error) {
	id, err := thread.Cast(req.Body.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	d, err := s.getDB(ctx, id, auth)
	if err != nil {
		return nil, err
	}
	cc, err := collectionConfigFromPb(req.Body.Config)
	if err != nil {
		return nil, err
	}
	if _, err = d.NewCollection(cc); err != nil {
		return nil, err
	}
	return &pb.NewCollectionReply{}, nil
}

// Create adds a new instance to a collection.
func (s *Service) Create(ctx context.Context, req *pb.CreateRequest) (*pb.CreateReply, error) {
	id, err := thread.Cast(req.Body.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	collection, err := s.getCollection(ctx, req.Body.CollectionName, id, auth)
	if err != nil {
		return nil, err
	}
	return s.processCreateRequest(req, collection.CreateMany)
}

// Save saves instances.
func (s *Service) Save(ctx context.Context, req *pb.SaveRequest) (*pb.SaveReply, error) {
	id, err := thread.Cast(req.Body.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	collection, err := s.getCollection(ctx, req.Body.CollectionName, id, auth)
	if err != nil {
		return nil, err
	}
	return s.processSaveRequest(req, collection.SaveMany)
}

// Delete deletes instances.
func (s *Service) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteReply, error) {
	id, err := thread.Cast(req.Body.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	collection, err := s.getCollection(ctx, req.Body.CollectionName, id, auth)
	if err != nil {
		return nil, err
	}
	return s.processDeleteRequest(req, collection.DeleteMany)
}

// Has determines if the collection inclides instances with the specified ids.
func (s *Service) Has(ctx context.Context, req *pb.HasRequest) (*pb.HasReply, error) {
	id, err := thread.Cast(req.Body.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	collection, err := s.getCollection(ctx, req.Body.CollectionName, id, auth)
	if err != nil {
		return nil, err
	}
	return s.processHasRequest(req, collection.HasMany)
}

// Find executes a query against the db.
func (s *Service) Find(ctx context.Context, req *pb.FindRequest) (*pb.FindReply, error) {
	id, err := thread.Cast(req.Body.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	collection, err := s.getCollection(ctx, req.Body.CollectionName, id, auth)
	if err != nil {
		return nil, err
	}
	return s.processFindRequest(req, collection.Find)
}

// FindByID searces for an instance by id.
func (s *Service) FindByID(ctx context.Context, req *pb.FindByIDRequest) (*pb.FindByIDReply, error) {
	id, err := thread.Cast(req.Body.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	collection, err := s.getCollection(ctx, req.Body.CollectionName, id, auth)
	if err != nil {
		return nil, err
	}
	return s.processFindByIDRequest(req, collection.FindByID)
}

// ReadTransaction runs a read transaction.
func (s *Service) ReadTransaction(stream pb.API_ReadTransactionServer) error {
	firstReq, err := stream.Recv()
	if err != nil {
		return err
	}

	var id thread.ID
	var auth *thread.Auth
	var collectionName string
	switch x := firstReq.Option.(type) {
	case *pb.ReadTransactionRequest_StartTransactionRequest:
		id, err = thread.Cast(x.StartTransactionRequest.Body.DbID)
		if err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		auth, err = getAuth(x.StartTransactionRequest.Header, x.StartTransactionRequest.Body)
		if err != nil {
			return err
		}
		collectionName = x.StartTransactionRequest.Body.CollectionName
	case nil:
		return fmt.Errorf("no ReadTransactionRequest type set")
	default:
		return fmt.Errorf("ReadTransactionRequest.Option has unexpected type %T", x)
	}

	collection, err := s.getCollection(stream.Context(), collectionName, id, auth)
	if err != nil {
		return err
	}

	return collection.ReadTxn(func(txn *db.Txn) error {
		for {
			req, err := stream.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			switch x := req.Option.(type) {
			case *pb.ReadTransactionRequest_HasRequest:
				innerReply, err := s.processHasRequest(x.HasRequest, func(ids []core.InstanceID, _ ...db.TxnOption) (bool, error) {
					return txn.Has(ids...)
				})
				if err != nil {
					return err
				}
				option := &pb.ReadTransactionReply_HasReply{HasReply: innerReply}
				if err := stream.Send(&pb.ReadTransactionReply{Option: option}); err != nil {
					return err
				}
			case *pb.ReadTransactionRequest_FindByIDRequest:
				innerReply, err := s.processFindByIDRequest(x.FindByIDRequest, func(id core.InstanceID, _ ...db.TxnOption) ([]byte, error) {
					return txn.FindByID(id)
				})
				if err != nil {
					return err
				}
				option := &pb.ReadTransactionReply_FindByIDReply{FindByIDReply: innerReply}
				if err := stream.Send(&pb.ReadTransactionReply{Option: option}); err != nil {
					return err
				}
			case *pb.ReadTransactionRequest_FindRequest:
				innerReply, err := s.processFindRequest(x.FindRequest, func(q *db.Query, _ ...db.TxnOption) (ret [][]byte, err error) {
					return txn.Find(q)
				})
				if err != nil {
					return err
				}
				option := &pb.ReadTransactionReply_FindReply{FindReply: innerReply}
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

// WriteTransaction runs a write transaction.
func (s *Service) WriteTransaction(stream pb.API_WriteTransactionServer) error {
	firstReq, err := stream.Recv()
	if err != nil {
		return err
	}

	var id thread.ID
	var auth *thread.Auth
	var collectionName string
	switch x := firstReq.Option.(type) {
	case *pb.WriteTransactionRequest_StartTransactionRequest:
		id, err = thread.Cast(x.StartTransactionRequest.Body.DbID)
		if err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		auth, err = getAuth(x.StartTransactionRequest.Header, x.StartTransactionRequest.Body)
		if err != nil {
			return err
		}
		collectionName = x.StartTransactionRequest.Body.CollectionName
	case nil:
		return fmt.Errorf("no WriteTransactionRequest type set")
	default:
		return fmt.Errorf("WriteTransactionRequest.Option has unexpected type %T", x)
	}

	collection, err := s.getCollection(stream.Context(), collectionName, id, auth)
	if err != nil {
		return err
	}

	return collection.WriteTxn(func(txn *db.Txn) error {
		for {
			req, err := stream.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			switch x := req.Option.(type) {
			case *pb.WriteTransactionRequest_HasRequest:
				innerReply, err := s.processHasRequest(x.HasRequest, func(ids []core.InstanceID, _ ...db.TxnOption) (bool, error) {
					return txn.Has(ids...)
				})
				if err != nil {
					return err
				}
				option := &pb.WriteTransactionReply_HasReply{HasReply: innerReply}
				if err := stream.Send(&pb.WriteTransactionReply{Option: option}); err != nil {
					return err
				}
			case *pb.WriteTransactionRequest_FindByIDRequest:
				innerReply, err := s.processFindByIDRequest(x.FindByIDRequest, func(id core.InstanceID, _ ...db.TxnOption) ([]byte, error) {
					return txn.FindByID(id)
				})
				if err != nil {
					return err
				}
				option := &pb.WriteTransactionReply_FindByIDReply{FindByIDReply: innerReply}
				if err := stream.Send(&pb.WriteTransactionReply{Option: option}); err != nil {
					return err
				}
			case *pb.WriteTransactionRequest_FindRequest:
				innerReply, err := s.processFindRequest(x.FindRequest, func(q *db.Query, _ ...db.TxnOption) (ret [][]byte, err error) {
					return txn.Find(q)
				})
				if err != nil {
					return err
				}
				option := &pb.WriteTransactionReply_FindReply{FindReply: innerReply}
				if err := stream.Send(&pb.WriteTransactionReply{Option: option}); err != nil {
					return err
				}
			case *pb.WriteTransactionRequest_CreateRequest:
				innerReply, err := s.processCreateRequest(x.CreateRequest, func(new [][]byte, _ ...db.TxnOption) ([]core.InstanceID, error) {
					return txn.Create(new...)
				})
				if err != nil {
					return err
				}
				option := &pb.WriteTransactionReply_CreateReply{CreateReply: innerReply}
				if err := stream.Send(&pb.WriteTransactionReply{Option: option}); err != nil {
					return err
				}
			case *pb.WriteTransactionRequest_SaveRequest:
				innerReply, err := s.processSaveRequest(x.SaveRequest, func(ids [][]byte, _ ...db.TxnOption) error {
					return txn.Save(ids...)
				})
				if err != nil {
					return err
				}
				option := &pb.WriteTransactionReply_SaveReply{SaveReply: innerReply}
				if err := stream.Send(&pb.WriteTransactionReply{Option: option}); err != nil {
					return err
				}
			case *pb.WriteTransactionRequest_DeleteRequest:
				innerReply, err := s.processDeleteRequest(x.DeleteRequest, func(ids []core.InstanceID, _ ...db.TxnOption) error {
					return txn.Delete(ids...)
				})
				if err != nil {
					return err
				}
				option := &pb.WriteTransactionReply_DeleteReply{DeleteReply: innerReply}
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

// Listen returns a stream of instances, trigged by a local or remote state change.
func (s *Service) Listen(req *pb.ListenRequest, server pb.API_ListenServer) error {
	id, err := thread.Cast(req.Body.DbID)
	if err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return err
	}
	d, err := s.getDB(server.Context(), id, auth)
	if err != nil {
		return err
	}

	options := make([]db.ListenOption, len(req.Body.Filters))
	for i, filter := range req.Body.Filters {
		var listenActionType db.ListenActionType
		switch filter.Action {
		case pb.ListenRequest_Body_Filter_ALL:
			listenActionType = db.ListenAll
		case pb.ListenRequest_Body_Filter_CREATE:
			listenActionType = db.ListenCreate
		case pb.ListenRequest_Body_Filter_DELETE:
			listenActionType = db.ListenDelete
		case pb.ListenRequest_Body_Filter_SAVE:
			listenActionType = db.ListenSave
		default:
			return status.Errorf(codes.InvalidArgument, "invalid filter action %v", filter.Action)
		}
		options[i] = db.ListenOption{
			Type:       listenActionType,
			Collection: filter.CollectionName,
			ID:         core.InstanceID(filter.InstanceID),
		}
	}

	l, err := d.Listen(options...)
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
			var instance []byte
			switch action.Type {
			case db.ActionCreate:
				replyAction = pb.ListenReply_CREATE
				instance, err = s.instanceForAction(d, action)
			case db.ActionDelete:
				replyAction = pb.ListenReply_DELETE
			case db.ActionSave:
				replyAction = pb.ListenReply_SAVE
				instance, err = s.instanceForAction(d, action)
			default:
				err = status.Errorf(codes.Internal, "unknown action type %v", action.Type)
			}
			if err != nil {
				return err
			}
			reply := &pb.ListenReply{
				CollectionName: action.Collection,
				InstanceID:     action.ID.String(),
				Action:         replyAction,
				Instance:       instance,
			}
			if err := server.Send(reply); err != nil {
				return err
			}
		}
	}
}

func (s *Service) instanceForAction(db *db.DB, action db.Action) ([]byte, error) {
	collection := db.GetCollection(action.Collection)
	if collection == nil {
		return nil, status.Error(codes.NotFound, "collection not found")
	}
	res, err := collection.FindByID(action.ID)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *Service) processCreateRequest(req *pb.CreateRequest, createFunc func([][]byte, ...db.TxnOption) ([]core.InstanceID, error)) (*pb.CreateReply, error) {
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	res, err := createFunc(req.Body.Instances, db.WithTxnAuth(auth))
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(res))
	for i, id := range res {
		ids[i] = id.String()
	}
	reply := &pb.CreateReply{
		InstanceIDs: ids,
	}
	return reply, nil
}

func (s *Service) processSaveRequest(req *pb.SaveRequest, saveFunc func([][]byte, ...db.TxnOption) error) (*pb.SaveReply, error) {
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	if err := saveFunc(req.Body.Instances, db.WithTxnAuth(auth)); err != nil {
		return nil, err
	}
	return &pb.SaveReply{}, nil
}

func (s *Service) processDeleteRequest(req *pb.DeleteRequest, deleteFunc func([]core.InstanceID, ...db.TxnOption) error) (*pb.DeleteReply, error) {
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	instanceIDs := make([]core.InstanceID, len(req.Body.InstanceIDs))
	for i, ID := range req.Body.InstanceIDs {
		instanceIDs[i] = core.InstanceID(ID)
	}
	if err := deleteFunc(instanceIDs, db.WithTxnAuth(auth)); err != nil {
		return nil, err
	}
	return &pb.DeleteReply{}, nil
}

func (s *Service) processHasRequest(req *pb.HasRequest, hasFunc func([]core.InstanceID, ...db.TxnOption) (bool, error)) (*pb.HasReply, error) {
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	instanceIDs := make([]core.InstanceID, len(req.Body.InstanceIDs))
	for i, ID := range req.Body.InstanceIDs {
		instanceIDs[i] = core.InstanceID(ID)
	}
	exists, err := hasFunc(instanceIDs, db.WithTxnAuth(auth))
	if err != nil {
		return nil, err
	}
	return &pb.HasReply{Exists: exists}, nil
}

func (s *Service) processFindByIDRequest(req *pb.FindByIDRequest, findFunc func(id core.InstanceID, opts ...db.TxnOption) ([]byte, error)) (*pb.FindByIDReply, error) {
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	instanceID := core.InstanceID(req.Body.InstanceID)
	found, err := findFunc(instanceID, db.WithTxnAuth(auth))
	if err != nil {
		return nil, err
	}
	return &pb.FindByIDReply{Instance: found}, nil
}

func (s *Service) processFindRequest(req *pb.FindRequest, findFunc func(q *db.Query, opts ...db.TxnOption) (ret [][]byte, err error)) (*pb.FindReply, error) {
	auth, err := getAuth(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	q := &db.Query{}
	if err := json.Unmarshal(req.Body.QueryJSON, q); err != nil {
		return nil, err
	}
	instances, err := findFunc(q, db.WithTxnAuth(auth))
	if err != nil {
		return nil, err
	}
	return &pb.FindReply{Instances: instances}, nil
}

func (s *Service) getDB(ctx context.Context, id thread.ID, auth *thread.Auth) (*db.DB, error) {
	d, err := s.manager.GetDB(ctx, id, db.WithManagedDBAuth(auth))
	if err != nil {
		if errors.Is(err, lstore.ErrThreadNotFound) {
			return nil, status.Error(codes.NotFound, "db not found")
		} else {
			return nil, err
		}
	}
	return d, nil
}

func (s *Service) getCollection(ctx context.Context, collectionName string, id thread.ID, auth *thread.Auth) (*db.Collection, error) {
	d, err := s.getDB(ctx, id, auth)
	if err != nil {
		return nil, err
	}
	collection := d.GetCollection(collectionName)
	if collection == nil {
		return nil, status.Error(codes.NotFound, "collection not found")
	}
	return collection, nil
}

func getAuth(header *pb.Header, body proto.Message) (*thread.Auth, error) {
	if header.PubKey == nil {
		return nil, nil
	}
	pk, err := crypto.UnmarshalPublicKey(header.PubKey)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	auth := thread.NewAuthFromSignature(pk, header.Signature)
	msg, err := proto.Marshal(body)
	if err != nil {
		return nil, err
	}
	_, _, err = auth.Sign(msg) // Auth is already signed, but this will set the message for verification
	return auth, err
}
