package net

import (
	"bytes"
	"context"
	"errors"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/status"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/cbor"
	lstore "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
	pb "github.com/textileio/go-threads/net/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// server implements the net gRPC server.
type server struct {
	sync.Mutex
	net   *net
	ps    *PubSub
	opts  []grpc.DialOption
	conns map[peer.ID]*grpc.ClientConn
}

// newServer creates a new network server.
func newServer(n *net, enablePubSub bool, opts ...grpc.DialOption) (*server, error) {
	var (
		s = &server{
			net:   n,
			conns: make(map[peer.ID]*grpc.ClientConn),
		}

		defaultOpts = []grpc.DialOption{
			s.getLibp2pDialer(),
			grpc.WithInsecure(),
		}
	)

	s.opts = append(defaultOpts, opts...)

	if enablePubSub {
		ps, err := pubsub.NewGossipSub(
			n.ctx,
			n.host,
			pubsub.WithMessageSigning(false),
			pubsub.WithStrictSignatureVerification(false))
		if err != nil {
			return nil, err
		}
		s.ps = NewPubSub(n.ctx, n.host.ID(), ps, s.pubsubHandler)

		ts, err := n.store.Threads()
		if err != nil {
			return nil, err
		}
		for _, id := range ts {
			if err := s.ps.Add(id); err != nil {
				return nil, err
			}
		}
	}

	return s, nil
}

// pubsubHandler receives records over pubsub.
func (s *server) pubsubHandler(ctx context.Context, req *pb.PushRecordRequest) {
	if _, err := s.PushRecord(ctx, req); err != nil {
		// This error will be "log not found" if the record sent over pubsub
		// beat the log, which has to be sent directly via the normal API.
		// In this case, the record will arrive directly after the log via
		// the normal API.
		log.Debugf("error handling pubsub record: %s", err)
	}
}

// GetLogs receives a get logs request.
func (s *server) GetLogs(_ context.Context, req *pb.GetLogsRequest) (*pb.GetLogsReply, error) {
	pid, err := verifyRequest(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	log.Debugf("received get logs request from %s", pid)

	pblgs := &pb.GetLogsReply{}
	if err := s.checkServiceKey(req.Body.ThreadID.ID, req.Body.ServiceKey); err != nil {
		return pblgs, err
	}

	info, err := s.net.store.GetThread(req.Body.ThreadID.ID) // Safe since putRecord will change head when fully-available
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	pblgs.Logs = make([]*pb.Log, len(info.Logs))
	for i, l := range info.Logs {
		pblgs.Logs[i] = logToProto(l)
	}

	log.Debugf("sending %d logs to %s", len(info.Logs), pid)

	return pblgs, nil
}

// PushLog receives a push log request.
// @todo: Don't overwrite info from non-owners
func (s *server) PushLog(_ context.Context, req *pb.PushLogRequest) (*pb.PushLogReply, error) {
	pid, err := verifyRequest(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	log.Debugf("received push log request from %s", pid)

	// Pick up missing keys
	info, err := s.net.store.GetThread(req.Body.ThreadID.ID)
	if err != nil && !errors.Is(err, lstore.ErrThreadNotFound) {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !info.Key.Defined() {
		if req.Body.ServiceKey != nil && req.Body.ServiceKey.Key != nil {
			if err = s.net.store.AddServiceKey(req.Body.ThreadID.ID, req.Body.ServiceKey.Key); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		} else {
			return nil, status.Error(codes.NotFound, lstore.ErrThreadNotFound.Error())
		}
	} else if !info.Key.CanRead() {
		if req.Body.ReadKey != nil && req.Body.ReadKey.Key != nil {
			if err = s.net.store.AddReadKey(req.Body.ThreadID.ID, req.Body.ReadKey.Key); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
	}

	lg := logFromProto(req.Body.Log)
	err = s.net.createExternalLogIfNotExist(req.Body.ThreadID.ID, lg.ID, lg.PubKey, lg.PrivKey, lg.Addrs)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	go s.net.updateRecordsFromLog(req.Body.ThreadID.ID, lg.ID)
	return &pb.PushLogReply{}, nil
}

// GetRecords receives a get records request.
func (s *server) GetRecords(ctx context.Context, req *pb.GetRecordsRequest) (*pb.GetRecordsReply, error) {
	pid, err := verifyRequest(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	log.Debugf("received get records request from %s", pid)

	pbrecs := &pb.GetRecordsReply{}
	if err := s.checkServiceKey(req.Body.ThreadID.ID, req.Body.ServiceKey); err != nil {
		return pbrecs, err
	}

	reqd := make(map[peer.ID]*pb.GetRecordsRequest_Body_LogEntry)
	for _, l := range req.Body.Logs {
		reqd[l.LogID.ID] = l
	}
	info, err := s.net.store.GetThread(req.Body.ThreadID.ID)
	if err != nil {
		return nil, err
	}
	pbrecs.Logs = make([]*pb.GetRecordsReply_LogEntry, len(info.Logs))

	for i, lg := range info.Logs {
		var offset cid.Cid
		var limit int
		var pblg *pb.Log
		if opts, ok := reqd[lg.ID]; ok {
			offset = opts.Offset.Cid
			limit = int(opts.Limit)
		} else {
			offset = cid.Undef
			limit = MaxPullLimit
			pblg = logToProto(lg)
		}
		recs, err := s.net.getLocalRecords(ctx, req.Body.ThreadID.ID, lg.ID, offset, limit)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		entry := &pb.GetRecordsReply_LogEntry{
			LogID:   &pb.ProtoPeerID{ID: lg.ID},
			Records: make([]*pb.Log_Record, len(recs)),
			Log:     pblg,
		}
		for j, r := range recs {
			entry.Records[j], err = cbor.RecordToProto(ctx, s.net, r)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
		pbrecs.Logs[i] = entry

		log.Debugf("sending %d records in log %s to %s", len(recs), lg.ID, pid)
	}

	return pbrecs, nil
}

// PushRecord receives a push record request.
func (s *server) PushRecord(ctx context.Context, req *pb.PushRecordRequest) (*pb.PushRecordReply, error) {
	pid, err := verifyRequest(req.Header, req.Body)
	if err != nil {
		return nil, err
	}
	log.Debugf("received push record request from %s", pid)

	// A log is required to accept new records
	logpk, err := s.net.store.PubKey(req.Body.ThreadID.ID, req.Body.LogID.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if logpk == nil {
		return nil, status.Error(codes.NotFound, "log not found")
	}

	key, err := s.net.store.ServiceKey(req.Body.ThreadID.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	rec, err := cbor.RecordFromProto(req.Body.Record, key)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	knownRecord, err := s.net.bstore.Has(rec.Cid())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if knownRecord {
		return &pb.PushRecordReply{}, nil
	}

	if err = rec.Verify(logpk); err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	if err = s.net.PutRecord(ctx, req.Body.ThreadID.ID, req.Body.LogID.ID, rec); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.PushRecordReply{}, nil
}

// checkServiceKey compares a key with the one stored under thread.
func (s *server) checkServiceKey(id thread.ID, k *pb.ProtoKey) error {
	if k == nil || k.Key == nil {
		return status.Error(codes.Unauthenticated, "a service-key is required to get logs")
	}
	sk, err := s.net.store.ServiceKey(id)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if sk == nil {
		return status.Error(codes.NotFound, lstore.ErrThreadNotFound.Error())
	}
	if !bytes.Equal(k.Key.Bytes(), sk.Bytes()) {
		return status.Error(codes.Unauthenticated, "invalid service-key")
	}
	return nil
}

// verifyRequest verifies that the signature associated with a request is valid.
func verifyRequest(header *pb.Header, body proto.Marshaler) (pid peer.ID, err error) {
	if header == nil || body == nil {
		err = status.Error(codes.InvalidArgument, "bad request")
		return
	}
	payload, err := body.Marshal()
	if err != nil {
		err = status.Error(codes.Internal, err.Error())
		return
	}
	ok, err := header.PubKey.Verify(payload, header.Signature)
	if !ok || err != nil {
		err = status.Error(codes.Unauthenticated, "bad signature")
		return
	}
	pid, err = peer.IDFromPublicKey(header.PubKey)
	if err != nil {
		err = status.Error(codes.InvalidArgument, err.Error())
		return
	}
	return pid, nil
}

// logToProto returns a proto log from a thread log.
func logToProto(l thread.LogInfo) *pb.Log {
	return &pb.Log{
		ID:     &pb.ProtoPeerID{ID: l.ID},
		PubKey: &pb.ProtoPubKey{PubKey: l.PubKey},
		Addrs:  addrsToProto(l.Addrs),
		Head:   &pb.ProtoCid{Cid: l.Head},
	}
}

// logFromProto returns a thread log from a proto log.
func logFromProto(l *pb.Log) thread.LogInfo {
	return thread.LogInfo{
		ID:     l.ID.ID,
		PubKey: l.PubKey.PubKey,
		Addrs:  addrsFromProto(l.Addrs),
		Head:   l.Head.Cid,
	}
}

func addrsToProto(mas []ma.Multiaddr) []pb.ProtoAddr {
	pas := make([]pb.ProtoAddr, len(mas))
	for i, a := range mas {
		pas[i] = pb.ProtoAddr{Multiaddr: a}
	}
	return pas
}

func addrsFromProto(pa []pb.ProtoAddr) []ma.Multiaddr {
	mas := make([]ma.Multiaddr, len(pa))
	for i, a := range pa {
		mas[i] = a.Multiaddr
	}
	return mas
}
