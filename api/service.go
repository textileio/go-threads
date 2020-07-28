package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/alecthomas/jsonschema"
	logging "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
	pb "github.com/textileio/go-threads/api/pb"
	"github.com/textileio/go-threads/core/app"
	core "github.com/textileio/go-threads/core/db"
	lstore "github.com/textileio/go-threads/core/logstore"
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

// NewService starts and returns a new service with the given network.
// The network is *not* managed by the server.
func NewService(network app.Net, conf Config) (*Service, error) {
	var err error
	if conf.Debug {
		err = util.SetLogLevels(map[string]logging.LogLevel{
			"threadsapi": logging.LevelDebug,
		})
		if err != nil {
			return nil, err
		}
	}

	manager, err := db.NewManager(network, db.WithNewRepoPath(conf.RepoPath), db.WithNewDebug(conf.Debug))
	if err != nil {
		return nil, err
	}
	return &Service{manager: manager}, nil
}

func (s *Service) Close() error {
	return s.manager.Close()
}

type remoteIdentity struct {
	pk     thread.PubKey
	server pb.API_GetTokenServer
}

func (i *remoteIdentity) MarshalBinary() ([]byte, error) {
	return nil, nil
}

func (i *remoteIdentity) UnmarshalBinary([]byte) error {
	return nil
}

func (i *remoteIdentity) Sign(ctx context.Context, msg []byte) ([]byte, error) {
	if err := i.server.Send(&pb.GetTokenReply{
		Payload: &pb.GetTokenReply_Challenge{
			Challenge: msg,
		},
	}); err != nil {
		return nil, err
	}

	var req *pb.GetTokenRequest
	done := make(chan error)
	go func() {
		defer close(done)
		var err error
		req, err = i.server.Recv()
		if err != nil {
			done <- err
			return
		}
	}()
	select {
	case <-ctx.Done():
		return nil, status.Error(codes.DeadlineExceeded, "Challenge deadline exceeded")
	case err, ok := <-done:
		if ok {
			return nil, err
		}
	}

	var sig []byte
	switch payload := req.Payload.(type) {
	case *pb.GetTokenRequest_Signature:
		sig = payload.Signature
	default:
		return nil, status.Error(codes.InvalidArgument, "Signature is required")
	}
	return sig, nil
}

func (i *remoteIdentity) GetPublic() thread.PubKey {
	return i.pk
}

func (i *remoteIdentity) Decrypt(context.Context, []byte) ([]byte, error) {
	return nil, nil // no-op
}

func (s *Service) GetToken(server pb.API_GetTokenServer) error {
	log.Debugf("received get token request")

	req, err := server.Recv()
	if err != nil {
		return err
	}
	key := &thread.Libp2pPubKey{}
	switch payload := req.Payload.(type) {
	case *pb.GetTokenRequest_Key:
		err = key.UnmarshalString(payload.Key)
		if err != nil {
			return err
		}
	default:
		return status.Error(codes.InvalidArgument, "Key is required")
	}

	identity := &remoteIdentity{
		pk:     key,
		server: server,
	}
	tok, err := s.manager.GetToken(server.Context(), identity)
	if err != nil {
		return err
	}
	return server.Send(&pb.GetTokenReply{
		Payload: &pb.GetTokenReply_Token{
			Token: string(tok),
		},
	})
}

func (s *Service) NewDB(ctx context.Context, req *pb.NewDBRequest) (*pb.NewDBReply, error) {
	log.Debugf("received new db request")

	id, err := thread.Cast(req.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	collections := make([]db.CollectionConfig, len(req.Collections))
	for i, c := range req.Collections {
		cc, err := collectionConfigFromPb(c)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		collections[i] = cc
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	if _, err = s.manager.NewDB(
		ctx,
		id,
		db.WithNewManagedName(req.Name),
		db.WithNewManagedToken(token),
		db.WithNewManagedCollections(collections...)); err != nil {
		return nil, err
	}
	return &pb.NewDBReply{}, nil
}

func (s *Service) NewDBFromAddr(ctx context.Context, req *pb.NewDBFromAddrRequest) (*pb.NewDBReply, error) {
	log.Debugf("received new db from address request")

	addr, err := ma.NewMultiaddrBytes(req.Addr)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	key, err := thread.KeyFromBytes(req.Key)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	collections := make([]db.CollectionConfig, len(req.Collections))
	for i, c := range req.Collections {
		cc, err := collectionConfigFromPb(c)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		collections[i] = cc
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	if _, err = s.manager.NewDBFromAddr(
		ctx,
		addr,
		key,
		db.WithNewManagedName(req.Name),
		db.WithNewManagedToken(token),
		db.WithNewManagedCollections(collections...),
		db.WithNewManagedBackfillBlock(req.Block)); err != nil {
		return nil, err
	}
	return &pb.NewDBReply{}, nil
}

func collectionConfigFromPb(pbc *pb.CollectionConfig) (db.CollectionConfig, error) {
	indexes := make([]db.Index, len(pbc.Indexes))
	for i, index := range pbc.Indexes {
		indexes[i] = db.Index{
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

func (s *Service) ListDBs(ctx context.Context, _ *pb.ListDBsRequest) (*pb.ListDBsReply, error) {
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}

	dbs, err := s.manager.ListDBs(ctx, db.WithManagedToken(token))
	if err != nil {
		return nil, err
	}
	pbdbs := make([]*pb.ListDBsReply_DB, len(dbs))
	var i int
	for id, d := range dbs {
		info, err := dBInfoToPb(d, token)
		if err != nil {
			return nil, err
		}
		pbdbs[i] = &pb.ListDBsReply_DB{
			DbID: id.Bytes(),
			Info: info,
		}
		i++
	}
	return &pb.ListDBsReply{Dbs: pbdbs}, nil
}

func (s *Service) GetDBInfo(ctx context.Context, req *pb.GetDBInfoRequest) (*pb.GetDBInfoReply, error) {
	id, err := thread.Cast(req.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}

	d, err := s.getDB(ctx, id, token)
	if err != nil {
		return nil, err
	}
	return dBInfoToPb(d, token)
}

func dBInfoToPb(d *db.DB, token thread.Token) (*pb.GetDBInfoReply, error) {
	addrs, key, err := d.GetDBInfo(db.WithToken(token))
	if err != nil {
		return nil, err
	}
	res := make([][]byte, len(addrs))
	for i := range addrs {
		res[i] = addrs[i].Bytes()
	}
	return &pb.GetDBInfoReply{
		Name:  d.GetName(),
		Addrs: res,
		Key:   key.Bytes(),
	}, nil
}

func (s *Service) DeleteDB(ctx context.Context, req *pb.DeleteDBRequest) (*pb.DeleteDBReply, error) {
	id, err := thread.Cast(req.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}

	if err = s.manager.DeleteDB(ctx, id, db.WithManagedToken(token)); err != nil {
		if errors.Is(err, lstore.ErrThreadNotFound) || errors.Is(err, db.ErrDBNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		} else {
			return nil, err
		}
	}
	return &pb.DeleteDBReply{}, nil
}

func (s *Service) NewCollection(ctx context.Context, req *pb.NewCollectionRequest) (*pb.NewCollectionReply, error) {
	id, err := thread.Cast(req.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	d, err := s.getDB(ctx, id, token)
	if err != nil {
		return nil, err
	}
	cc, err := collectionConfigFromPb(req.Config)
	if err != nil {
		return nil, err
	}
	if _, err = d.NewCollection(cc); err != nil {
		return nil, err
	}
	return &pb.NewCollectionReply{}, nil
}

func (s *Service) UpdateCollection(ctx context.Context, req *pb.UpdateCollectionRequest) (*pb.UpdateCollectionReply, error) {
	id, err := thread.Cast(req.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	d, err := s.getDB(ctx, id, token)
	if err != nil {
		return nil, err
	}
	cc, err := collectionConfigFromPb(req.Config)
	if err != nil {
		return nil, err
	}
	if _, err = d.UpdateCollection(cc); err != nil {
		return nil, err
	}
	return &pb.UpdateCollectionReply{}, nil
}

func (s *Service) DeleteCollection(ctx context.Context, req *pb.DeleteCollectionRequest) (*pb.DeleteCollectionReply, error) {
	id, err := thread.Cast(req.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	d, err := s.getDB(ctx, id, token)
	if err != nil {
		return nil, err
	}
	if err = d.DeleteCollection(req.Name); err != nil {
		return nil, err
	}
	return &pb.DeleteCollectionReply{}, nil
}

func (s *Service) GetCollectionInfo(ctx context.Context, req *pb.GetCollectionInfoRequest) (*pb.GetCollectionInfoReply, error) {
	id, err := thread.Cast(req.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	collection, err := s.getCollection(ctx, req.Name, id, token)
	if err != nil {
		return nil, err
	}
	return &pb.GetCollectionInfoReply{
		Name:    collection.GetName(),
		Schema:  collection.GetSchema(),
		Indexes: indexesToPb(collection.GetIndexes()),
	}, nil
}

func indexesToPb(indexes []db.Index) []*pb.Index {
	pbindexes := make([]*pb.Index, len(indexes))
	for i, index := range indexes {
		pbindexes[i] = &pb.Index{
			Path:   index.Path,
			Unique: index.Unique,
		}
	}
	return pbindexes
}

func (s *Service) GetCollectionIndexes(ctx context.Context, req *pb.GetCollectionIndexesRequest) (*pb.GetCollectionIndexesReply, error) {
	id, err := thread.Cast(req.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	collection, err := s.getCollection(ctx, req.Name, id, token)
	if err != nil {
		return nil, err
	}
	return &pb.GetCollectionIndexesReply{
		Indexes: indexesToPb(collection.GetIndexes()),
	}, nil
}

func (s *Service) ListCollections(ctx context.Context, req *pb.ListCollectionsRequest) (*pb.ListCollectionsReply, error) {
	id, err := thread.Cast(req.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	d, err := s.getDB(ctx, id, token)
	if err != nil {
		return nil, err
	}
	list := d.ListCollections(db.WithToken(token))
	pblist := make([]*pb.GetCollectionInfoReply, len(list))
	for i, c := range list {
		pblist[i] = &pb.GetCollectionInfoReply{
			Name:    c.GetName(),
			Schema:  c.GetSchema(),
			Indexes: indexesToPb(c.GetIndexes()),
		}
	}
	return &pb.ListCollectionsReply{Collections: pblist}, nil
}

func (s *Service) Create(ctx context.Context, req *pb.CreateRequest) (*pb.CreateReply, error) {
	id, err := thread.Cast(req.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	collection, err := s.getCollection(ctx, req.CollectionName, id, token)
	if err != nil {
		return nil, err
	}
	return s.processCreateRequest(req, token, collection.CreateMany)
}

func (s *Service) Save(ctx context.Context, req *pb.SaveRequest) (*pb.SaveReply, error) {
	id, err := thread.Cast(req.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	collection, err := s.getCollection(ctx, req.CollectionName, id, token)
	if err != nil {
		return nil, err
	}
	return s.processSaveRequest(req, token, collection.SaveMany)
}

func (s *Service) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteReply, error) {
	id, err := thread.Cast(req.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	collection, err := s.getCollection(ctx, req.CollectionName, id, token)
	if err != nil {
		return nil, err
	}
	return s.processDeleteRequest(req, token, collection.DeleteMany)
}

func (s *Service) Has(ctx context.Context, req *pb.HasRequest) (*pb.HasReply, error) {
	id, err := thread.Cast(req.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	collection, err := s.getCollection(ctx, req.CollectionName, id, token)
	if err != nil {
		return nil, err
	}
	return s.processHasRequest(req, token, collection.HasMany)
}

func (s *Service) Find(ctx context.Context, req *pb.FindRequest) (*pb.FindReply, error) {
	id, err := thread.Cast(req.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	collection, err := s.getCollection(ctx, req.CollectionName, id, token)
	if err != nil {
		return nil, err
	}
	return s.processFindRequest(req, token, collection.Find)
}

func (s *Service) FindByID(ctx context.Context, req *pb.FindByIDRequest) (*pb.FindByIDReply, error) {
	id, err := thread.Cast(req.DbID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return nil, err
	}
	collection, err := s.getCollection(ctx, req.CollectionName, id, token)
	if err != nil {
		return nil, err
	}
	return s.processFindByIDRequest(req, token, collection.FindByID)
}

func (s *Service) ReadTransaction(stream pb.API_ReadTransactionServer) error {
	firstReq, err := stream.Recv()
	if err != nil {
		return err
	}

	var id thread.ID
	var collectionName string
	switch x := firstReq.Option.(type) {
	case *pb.ReadTransactionRequest_StartTransactionRequest:
		id, err = thread.Cast(x.StartTransactionRequest.DbID)
		if err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		collectionName = x.StartTransactionRequest.CollectionName
	case nil:
		return fmt.Errorf("no ReadTransactionRequest type set")
	default:
		return fmt.Errorf("ReadTransactionRequest.Option has unexpected type %T", x)
	}

	token, err := thread.NewTokenFromMD(stream.Context())
	if err != nil {
		return err
	}
	collection, err := s.getCollection(stream.Context(), collectionName, id, token)
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
				innerReply, err := s.processHasRequest(x.HasRequest, token, func(ids []core.InstanceID, _ ...db.TxnOption) (bool, error) {
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
				innerReply, err := s.processFindByIDRequest(x.FindByIDRequest, token, func(id core.InstanceID, _ ...db.TxnOption) ([]byte, error) {
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
				innerReply, err := s.processFindRequest(x.FindRequest, token, func(q *db.Query, _ ...db.TxnOption) (ret [][]byte, err error) {
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
	}, db.WithTxnToken(token))
}

func (s *Service) WriteTransaction(stream pb.API_WriteTransactionServer) error {
	firstReq, err := stream.Recv()
	if err != nil {
		return err
	}

	var id thread.ID
	var collectionName string
	switch x := firstReq.Option.(type) {
	case *pb.WriteTransactionRequest_StartTransactionRequest:
		id, err = thread.Cast(x.StartTransactionRequest.DbID)
		if err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		collectionName = x.StartTransactionRequest.CollectionName
	case nil:
		return fmt.Errorf("no WriteTransactionRequest type set")
	default:
		return fmt.Errorf("WriteTransactionRequest.Option has unexpected type %T", x)
	}

	token, err := thread.NewTokenFromMD(stream.Context())
	if err != nil {
		return err
	}
	collection, err := s.getCollection(stream.Context(), collectionName, id, token)
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
				innerReply, err := s.processHasRequest(x.HasRequest, token, func(ids []core.InstanceID, _ ...db.TxnOption) (bool, error) {
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
				innerReply, err := s.processFindByIDRequest(x.FindByIDRequest, token, func(id core.InstanceID, _ ...db.TxnOption) ([]byte, error) {
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
				innerReply, err := s.processFindRequest(x.FindRequest, token, func(q *db.Query, _ ...db.TxnOption) (ret [][]byte, err error) {
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
				innerReply, err := s.processCreateRequest(x.CreateRequest, token, func(new [][]byte, _ ...db.TxnOption) ([]core.InstanceID, error) {
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
				innerReply, err := s.processSaveRequest(x.SaveRequest, token, func(ids [][]byte, _ ...db.TxnOption) error {
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
				innerReply, err := s.processDeleteRequest(x.DeleteRequest, token, func(ids []core.InstanceID, _ ...db.TxnOption) error {
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
	}, db.WithTxnToken(token))
}

func (s *Service) Listen(req *pb.ListenRequest, server pb.API_ListenServer) error {
	id, err := thread.Cast(req.DbID)
	if err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	token, err := thread.NewTokenFromMD(server.Context())
	if err != nil {
		return err
	}
	d, err := s.getDB(server.Context(), id, token)
	if err != nil {
		return err
	}

	options := make([]db.ListenOption, len(req.Filters))
	for i, filter := range req.Filters {
		var listenActionType db.ListenActionType
		switch filter.Action {
		case pb.ListenRequest_Filter_ALL:
			listenActionType = db.ListenAll
		case pb.ListenRequest_Filter_CREATE:
			listenActionType = db.ListenCreate
		case pb.ListenRequest_Filter_DELETE:
			listenActionType = db.ListenDelete
		case pb.ListenRequest_Filter_SAVE:
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

func (s *Service) instanceForAction(d *db.DB, action db.Action) ([]byte, error) {
	collection := d.GetCollection(action.Collection)
	if collection == nil {
		return nil, status.Error(codes.NotFound, db.ErrCollectionNotFound.Error())
	}
	res, err := collection.FindByID(action.ID)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *Service) processCreateRequest(req *pb.CreateRequest, token thread.Token, createFunc func([][]byte, ...db.TxnOption) ([]core.InstanceID, error)) (*pb.CreateReply, error) {
	res, err := createFunc(req.Instances, db.WithTxnToken(token))
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

func (s *Service) processSaveRequest(req *pb.SaveRequest, token thread.Token, saveFunc func([][]byte, ...db.TxnOption) error) (*pb.SaveReply, error) {
	if err := saveFunc(req.Instances, db.WithTxnToken(token)); err != nil {
		return nil, err
	}
	return &pb.SaveReply{}, nil
}

func (s *Service) processDeleteRequest(req *pb.DeleteRequest, token thread.Token, deleteFunc func([]core.InstanceID, ...db.TxnOption) error) (*pb.DeleteReply, error) {
	instanceIDs := make([]core.InstanceID, len(req.InstanceIDs))
	for i, ID := range req.InstanceIDs {
		instanceIDs[i] = core.InstanceID(ID)
	}
	if err := deleteFunc(instanceIDs, db.WithTxnToken(token)); err != nil {
		return nil, err
	}
	return &pb.DeleteReply{}, nil
}

func (s *Service) processHasRequest(req *pb.HasRequest, token thread.Token, hasFunc func([]core.InstanceID, ...db.TxnOption) (bool, error)) (*pb.HasReply, error) {
	instanceIDs := make([]core.InstanceID, len(req.InstanceIDs))
	for i, ID := range req.InstanceIDs {
		instanceIDs[i] = core.InstanceID(ID)
	}
	exists, err := hasFunc(instanceIDs, db.WithTxnToken(token))
	if err != nil {
		return nil, err
	}
	return &pb.HasReply{Exists: exists}, nil
}

func (s *Service) processFindByIDRequest(req *pb.FindByIDRequest, token thread.Token, findFunc func(id core.InstanceID, opts ...db.TxnOption) ([]byte, error)) (*pb.FindByIDReply, error) {
	instanceID := core.InstanceID(req.InstanceID)
	found, err := findFunc(instanceID, db.WithTxnToken(token))
	if err != nil {
		return nil, err
	}
	return &pb.FindByIDReply{Instance: found}, nil
}

func (s *Service) processFindRequest(req *pb.FindRequest, token thread.Token, findFunc func(q *db.Query, opts ...db.TxnOption) (ret [][]byte, err error)) (*pb.FindReply, error) {
	q := &db.Query{}
	if err := json.Unmarshal(req.QueryJSON, q); err != nil {
		return nil, err
	}
	instances, err := findFunc(q, db.WithTxnToken(token))
	if err != nil {
		return nil, err
	}
	return &pb.FindReply{Instances: instances}, nil
}

func (s *Service) getDB(ctx context.Context, id thread.ID, token thread.Token) (*db.DB, error) {
	d, err := s.manager.GetDB(ctx, id, db.WithManagedToken(token))
	if err != nil {
		if errors.Is(err, lstore.ErrThreadNotFound) || errors.Is(err, db.ErrDBNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		} else {
			return nil, err
		}
	}
	return d, nil
}

func (s *Service) getCollection(ctx context.Context, collectionName string, id thread.ID, token thread.Token) (*db.Collection, error) {
	d, err := s.getDB(ctx, id, token)
	if err != nil {
		return nil, err
	}
	collection := d.GetCollection(collectionName)
	if collection == nil {
		return nil, status.Error(codes.NotFound, db.ErrCollectionNotFound.Error())
	}
	return collection, nil
}
