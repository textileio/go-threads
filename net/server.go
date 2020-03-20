package net

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/gogo/status"
	"github.com/ipfs/go-cid"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/cbor"
	lstore "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
	pb "github.com/textileio/go-threads/net/pb"
	"google.golang.org/grpc/codes"
)

// server implements the net gRPC server.
type server struct {
	net *net
	ps  *PubSub
}

// newServer creates a new network server.
func newServer(n *net) (*server, error) {
	s := &server{net: n}

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
// @todo: Verification
func (s *server) GetLogs(_ context.Context, req *pb.GetLogsRequest) (*pb.GetLogsReply, error) {
	if req.Header == nil {
		return nil, status.Error(codes.FailedPrecondition, "request header is required")
	}
	log.Debugf("received get logs request from %s", req.Header.From.ID)

	pblgs := &pb.GetLogsReply{}

	if err := s.checkServiceKey(req.ThreadID.ID, req.ServiceKey); err != nil {
		return pblgs, err
	}

	info, err := s.net.store.GetThread(req.ThreadID.ID) // Safe since putRecord will change head when fully-available
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	pblgs.Logs = make([]*pb.Log, len(info.Logs))
	for i, l := range info.Logs {
		pblgs.Logs[i] = logToProto(l)
	}

	log.Debugf("sending %d logs to %s", len(info.Logs), req.Header.From.ID)

	return pblgs, nil
}

// PushLog receives a push log request.
// @todo: Verification
// @todo: Don't overwrite info from non-owners
func (s *server) PushLog(_ context.Context, req *pb.PushLogRequest) (*pb.PushLogReply, error) {
	if req.Header == nil {
		return nil, status.Error(codes.FailedPrecondition, "request header is required")
	}
	log.Debugf("received push log request from %s", req.Header.From.ID)

	// Pick up missing keys
	info, err := s.net.store.GetThread(req.ThreadID.ID)
	if err != nil && !errors.Is(err, lstore.ErrThreadNotFound) {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !info.Key.Defined() {
		if req.ServiceKey != nil && req.ServiceKey.Key != nil {
			if err = s.net.store.AddServiceKey(req.ThreadID.ID, req.ServiceKey.Key); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		} else {
			return nil, status.Error(codes.NotFound, lstore.ErrThreadNotFound.Error())
		}
	} else if !info.Key.CanRead() {
		if req.ReadKey != nil && req.ReadKey.Key != nil {
			if err = s.net.store.AddReadKey(req.ThreadID.ID, req.ReadKey.Key); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
	}

	lg := logFromProto(req.Log)
	err = s.net.createExternalLogIfNotExist(req.ThreadID.ID, lg.ID, lg.PubKey, lg.PrivKey, lg.Addrs)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	go s.net.updateRecordsFromLog(req.ThreadID.ID, lg.ID)

	return &pb.PushLogReply{}, nil
}

// GetRecords receives a get records request.
// @todo: Verification
func (s *server) GetRecords(ctx context.Context, req *pb.GetRecordsRequest) (*pb.GetRecordsReply, error) {
	if req.Header == nil {
		return nil, status.Error(codes.FailedPrecondition, "request header is required")
	}
	log.Debugf("received get records request from %s", req.Header.From.ID)

	pbrecs := &pb.GetRecordsReply{}

	if err := s.checkServiceKey(req.ThreadID.ID, req.ServiceKey); err != nil {
		return pbrecs, err
	}

	reqd := make(map[peer.ID]*pb.GetRecordsRequest_LogEntry)
	for _, l := range req.Logs {
		reqd[l.LogID.ID] = l
	}
	info, err := s.net.store.GetThread(req.ThreadID.ID)
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
		recs, err := s.net.getLocalRecords(
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
			entry.Records[j], err = cbor.RecordToProto(ctx, s.net, r)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
		pbrecs.Logs[i] = entry

		log.Debugf("sending %d records in log %s to %s", len(recs), lg.ID, req.Header.From.ID)
	}

	return pbrecs, nil
}

// PushRecord receives a push record request.
func (s *server) PushRecord(ctx context.Context, req *pb.PushRecordRequest) (*pb.PushRecordReply, error) {
	if req.Header == nil {
		return nil, status.Error(codes.FailedPrecondition, "request header is required")
	}
	log.Debugf("received push record request from %s", req.Header.From.ID)

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
	logpk, err := s.net.store.PubKey(req.ThreadID.ID, req.LogID.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if logpk == nil {
		return nil, status.Error(codes.NotFound, "log not found")
	}

	key, err := s.net.store.ServiceKey(req.ThreadID.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	rec, err := cbor.RecordFromProto(req.Record, key)
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

	// Verify node
	if err = rec.Verify(logpk); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err = s.net.PutRecord(ctx, req.ThreadID.ID, req.LogID.ID, rec); err != nil {
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
		return status.Error(codes.PermissionDenied, "invalid service-key")
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
	return &pb.Log{
		ID:     &pb.ProtoPeerID{ID: l.ID},
		PubKey: &pb.ProtoPubKey{PubKey: l.PubKey},
		Addrs:  pbaddrs,
		Head:   &pb.ProtoCid{Cid: l.Head},
	}
}

// logFromProto returns a thread log from a proto log.
func logFromProto(l *pb.Log) thread.LogInfo {
	addrs := make([]ma.Multiaddr, len(l.Addrs))
	for j, a := range l.Addrs {
		addrs[j] = a.Multiaddr
	}
	return thread.LogInfo{
		ID:     l.ID.ID,
		PubKey: l.PubKey.PubKey,
		Addrs:  addrs,
		Head:   l.Head.Cid,
	}
}
