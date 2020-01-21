package service

import (
	"bytes"
	"context"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/status"
	"github.com/ipfs/go-cid"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/cbor"
	"github.com/textileio/go-threads/core/thread"
	pb "github.com/textileio/go-threads/service/pb"
	"google.golang.org/grpc/codes"
)

// server implements the service gRPC server.
type server struct {
	threads *service
	pubsub  *pubsub.PubSub
}

// newServer creates a new service network server.
func newServer(t *service) (*server, error) {
	ps, err := pubsub.NewGossipSub(
		t.ctx,
		t.host,
		pubsub.WithMessageSigning(false),
		pubsub.WithStrictSignatureVerification(false))
	if err != nil {
		return nil, err
	}

	s := &server{
		threads: t,
		pubsub:  ps,
	}

	// @todo: clean up pubsub handling (we need to track the new-style topic handles)
	//ts, err := t.store.Threads()
	//if err != nil {
	//	return nil, err
	//}
	//for _, id := range ts {
	//	go s.subscribe(id)
	//}

	// @todo: ts.pubsub.RegisterTopicValidator()

	return s, nil
}

// GetLogs receives a get logs request.
// @todo: Verification
func (s *server) GetLogs(_ context.Context, req *pb.GetLogsRequest) (*pb.GetLogsReply, error) {
	if req.Header == nil {
		return nil, status.Error(codes.FailedPrecondition, "request header is required")
	}
	log.Debugf("received get logs request from %s", req.Header.From.ID.String())

	pblgs := &pb.GetLogsReply{}

	if err := s.checkFollowKey(req.ThreadID.ID, req.FollowKey); err != nil {
		return pblgs, err
	}

	info, err := s.threads.store.ThreadInfo(req.ThreadID.ID) // Safe since putRecord will change head when fully-available
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	pblgs.Logs = make([]*pb.Log, len(info.Logs))
	for i, l := range info.Logs {
		pblgs.Logs[i] = logToProto(l)
	}

	log.Debugf("sending %d logs to %s", len(info.Logs), req.Header.From.ID.String())

	return pblgs, nil
}

// PushLog receives a push log request.
// @todo: Verification
// @todo: Don't overwrite info from non-owners
func (s *server) PushLog(_ context.Context, req *pb.PushLogRequest) (*pb.PushLogReply, error) {
	if req.Header == nil {
		return nil, status.Error(codes.FailedPrecondition, "request header is required")
	}
	log.Debugf("received push log request from %s", req.Header.From.ID.String())

	// Pick up missing keys
	info, err := s.threads.store.ThreadInfo(req.ThreadID.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if info.FollowKey == nil {
		if req.FollowKey != nil && req.FollowKey.Key != nil {
			if err = s.threads.store.AddFollowKey(req.ThreadID.ID, req.FollowKey.Key); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		} else {
			return nil, status.Error(codes.NotFound, "thread not found")
		}
	}
	if info.ReadKey == nil {
		if req.ReadKey != nil && req.ReadKey.Key != nil {
			if err = s.threads.store.AddReadKey(req.ThreadID.ID, req.ReadKey.Key); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
	}

	lg := logFromProto(req.Log)
	err = s.threads.createExternalLogIfNotExist(req.ThreadID.ID, lg.ID, lg.PubKey, lg.PrivKey, lg.Addrs)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	go s.threads.updateRecordsFromLog(req.ThreadID.ID, lg.ID)

	return &pb.PushLogReply{}, nil
}

// GetRecords receives a get records request.
// @todo: Verification
func (s *server) GetRecords(ctx context.Context, req *pb.GetRecordsRequest) (*pb.GetRecordsReply, error) {
	if req.Header == nil {
		return nil, status.Error(codes.FailedPrecondition, "request header is required")
	}
	log.Debugf("received get records request from %s", req.Header.From.ID.String())

	pbrecs := &pb.GetRecordsReply{}

	if err := s.checkFollowKey(req.ThreadID.ID, req.FollowKey); err != nil {
		return pbrecs, err
	}

	reqd := make(map[peer.ID]*pb.GetRecordsRequest_LogEntry)
	for _, l := range req.Logs {
		reqd[l.LogID.ID] = l
	}
	info, err := s.threads.store.ThreadInfo(req.ThreadID.ID)
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
		recs, err := s.threads.getLocalRecords(
			ctx,
			req.ThreadID.ID,
			lg.ID,
			offset,
			limit)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		entry := &pb.GetRecordsReply_LogEntry{
			LogID:   &pb.ProtoPeerID{ID: lg.ID},
			Records: make([]*pb.Log_Record, len(recs)),
			Log:     pblg,
		}
		for j, r := range recs {
			entry.Records[j], err = cbor.RecordToProto(ctx, s.threads, r)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
		pbrecs.Logs[i] = entry

		log.Debugf("sending %d records in log %s to %s", len(recs), lg.ID.String(), req.Header.From.ID.String())
	}

	return pbrecs, nil
}

// PushRecord receives a push record request.
func (s *server) PushRecord(ctx context.Context, req *pb.PushRecordRequest) (*pb.PushRecordReply, error) {
	if req.Header == nil {
		return nil, status.Error(codes.FailedPrecondition, "request header is required")
	}
	log.Debugf("received push record request from %s", req.Header.From.ID.String())

	// Verify the request
	reqpk, err := requestPubKey(req)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	err = verifyRequestSignature(req.Record, reqpk, req.Header.Signature)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// A log is required to accept new records
	logpk, err := s.threads.store.PubKey(req.ThreadID.ID, req.LogID.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if logpk == nil {
		return nil, status.Error(codes.NotFound, "log not found")
	}

	key, err := s.threads.store.FollowKey(req.ThreadID.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	rec, err := cbor.RecordFromProto(req.Record, key)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	knownRecord, err := s.threads.bstore.Has(rec.Cid())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if knownRecord {
		return &pb.PushRecordReply{}, nil
	}

	// Verify node
	if err = rec.Verify(logpk); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err = s.threads.PutRecord(ctx, req.ThreadID.ID, req.LogID.ID, rec); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.PushRecordReply{}, nil
}

// subscribe to a thread for updates.
func (s *server) subscribe(id thread.ID) {
	sub, err := s.pubsub.Subscribe(id.String())
	if err != nil {
		log.Error(err)
		return
	}

	for {
		msg, err := sub.Next(s.threads.ctx)
		if err != nil {
			break
		}

		from, err := peer.IDFromBytes(msg.From)
		if err != nil {
			log.Error(err)
			break
		}
		if from.String() == s.threads.host.ID().String() {
			continue
		}

		req := new(pb.PushRecordRequest)
		err = proto.Unmarshal(msg.Data, req)
		if err != nil {
			log.Error(err)
			continue
		}

		log.Debugf("received multicast request from %s", from.String())

		_, err = s.PushRecord(s.threads.ctx, req)
		if err != nil {
			log.Warnf("pubsub: %s", err)
			continue
		}
	}
}

// checkFollowKey compares a key with the one stored under thread.
func (s *server) checkFollowKey(id thread.ID, pfk *pb.ProtoKey) error {
	if pfk == nil || pfk.Key == nil {
		return status.Error(codes.Unauthenticated, "a follow-key is required to get logs")
	}
	fk, err := s.threads.store.FollowKey(id)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if fk == nil {
		return status.Error(codes.NotFound, "thread not found")
	}
	if !bytes.Equal(pfk.Key.Bytes(), fk.Bytes()) {
		return status.Error(codes.PermissionDenied, "invalid follow-key")
	}
	return nil
}

// requestPubKey returns the key associated with a request.
func requestPubKey(r *pb.PushRecordRequest) (ic.PubKey, error) {
	var pubk ic.PubKey
	var err error
	if r.Header.Key == nil {
		// No attached key, it must be extractable from the source ID
		pubk, err = r.Header.From.ID.ExtractPublicKey()
		if err != nil {
			return nil, fmt.Errorf("cannot extract signing key: %s", err)
		}
		if pubk == nil {
			return nil, fmt.Errorf("cannot extract signing key")
		}
	} else {
		pubk = r.Header.Key.PubKey
		// Verify that the source ID matches the attached key
		if !r.Header.From.ID.MatchesPublicKey(r.Header.Key.PubKey) {
			return nil, fmt.Errorf(
				"bad signing key; source ID %s doesn't match key",
				r.Header.From.ID)
		}
	}
	return pubk, nil
}

// verifyRequestSignature verifies that the signature assocated with a request is valid.
func verifyRequestSignature(rec *pb.Log_Record, pk ic.PubKey, sig []byte) error {
	payload, err := rec.Marshal()
	if err != nil {
		return err
	}
	ok, err := pk.Verify(payload, sig)
	if !ok || err != nil {
		return fmt.Errorf("bad signature")
	}
	return nil
}

// logToProto returns a proto log from a thread log.
func logToProto(l thread.LogInfo) *pb.Log {
	pbaddrs := make([]pb.ProtoAddr, len(l.Addrs))
	for j, a := range l.Addrs {
		pbaddrs[j] = pb.ProtoAddr{Multiaddr: a}
	}
	pbheads := make([]pb.ProtoCid, len(l.Heads))
	for k, h := range l.Heads {
		pbheads[k] = pb.ProtoCid{Cid: h}
	}
	return &pb.Log{
		ID:     &pb.ProtoPeerID{ID: l.ID},
		PubKey: &pb.ProtoPubKey{PubKey: l.PubKey},
		Addrs:  pbaddrs,
		Heads:  pbheads,
	}
}

// logFromProto returns a thread log from a proto log.
func logFromProto(l *pb.Log) thread.LogInfo {
	addrs := make([]ma.Multiaddr, len(l.Addrs))
	for j, a := range l.Addrs {
		addrs[j] = a.Multiaddr
	}
	heads := make([]cid.Cid, len(l.Heads))
	for k, h := range l.Heads {
		heads[k] = h.Cid
	}
	return thread.LogInfo{
		ID:     l.ID.ID,
		PubKey: l.PubKey.PubKey,
		Addrs:  addrs,
		Heads:  heads,
	}
}
