package net

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/gogo/status"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/cbor"
	lstore "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/logstore/lstoreds"
	pb "github.com/textileio/go-threads/net/pb"
	"github.com/textileio/go-threads/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcpeer "google.golang.org/grpc/peer"
)

var (
	errNoAddrsEdge = errors.New("no addresses to compute edge")
	errNoHeadsEdge = errors.New("no heads to compute edge")
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
func (s *server) GetLogs(ctx context.Context, req *pb.GetLogsRequest) (*pb.GetLogsReply, error) {
	pid, err := peerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	log.Debugf("received get logs request from %s", pid)

	pblgs := &pb.GetLogsReply{}
	if err := s.checkServiceKey(req.Body.ThreadID.ID, req.Body.ServiceKey); err != nil {
		return pblgs, err
	}

	info, err := s.net.store.GetThread(req.Body.ThreadID.ID) // Safe since putRecords will change head when fully-available
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
func (s *server) PushLog(ctx context.Context, req *pb.PushLogRequest) (*pb.PushLogReply, error) {
	pid, err := peerIDFromContext(ctx)
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
	if err = s.net.createExternalLogsIfNotExist(req.Body.ThreadID.ID, []thread.LogInfo{lg}); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if s.net.queueGetRecords.Schedule(pid, req.Body.ThreadID.ID, callPriorityLow, s.net.updateRecordsFromPeer) {
		log.Debugf("record update for thread %s from %s scheduled", req.Body.ThreadID.ID, pid)
	}
	return &pb.PushLogReply{}, nil
}

// GetRecords receives a get records request.
func (s *server) GetRecords(ctx context.Context, req *pb.GetRecordsRequest) (*pb.GetRecordsReply, error) {
	pid, err := peerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	log.Debugf("received get records request from %s", pid)

	var pbrecs = &pb.GetRecordsReply{}
	if err := s.checkServiceKey(req.Body.ThreadID.ID, req.Body.ServiceKey); err != nil {
		return pbrecs, err
	}

	// fast check if requested offsets are equal with thread heads
	if changed, err := s.headsChanged(req); err != nil {
		return nil, err
	} else if !changed {
		return pbrecs, nil
	}

	reqd := make(map[peer.ID]*pb.GetRecordsRequest_Body_LogEntry)
	for _, l := range req.Body.Logs {
		reqd[l.LogID.ID] = l
	}
	info, err := s.net.store.GetThread(req.Body.ThreadID.ID)
	if err != nil {
		return nil, err
	} else if len(info.Logs) == 0 {
		return pbrecs, nil
	}
	pbrecs.Logs = make([]*pb.GetRecordsReply_LogEntry, 0, len(info.Logs))

	var (
		logRecordLimit = MaxPullLimit / len(info.Logs)
		mx             sync.Mutex
		wg             sync.WaitGroup
	)

	for _, lg := range info.Logs {
		var (
			offset  cid.Cid
			limit   int
			counter int64
			pblg    *pb.Log
		)
		if opts, ok := reqd[lg.ID]; ok {
			offset = opts.Offset.Cid
			counter = opts.Counter
			limit = minInt(int(opts.Limit), logRecordLimit)
		} else {
			offset = cid.Undef
			limit = logRecordLimit
			counter = thread.CounterUndef
		}
		pblg = logToProto(lg)

		wg.Add(1)
		go func(tid thread.ID, lid peer.ID, off cid.Cid, lim int) {
			defer wg.Done()
			// if we don't have records in the log then skipping it
			if pblg.Counter == thread.CounterUndef {
				return
			}

			recs, err := s.net.getLocalRecords(ctx, tid, lid, off, lim, counter)
			if err != nil {
				log.Errorf("getting local records (thread %s, log %s): %v", tid, lid, err)
			}

			var prs = make([]*pb.Log_Record, 0, len(recs))
			for _, r := range recs {
				pr, err := cbor.RecordToProto(ctx, s.net, r)
				if err != nil {
					log.Errorf("constructing proto-record %s (thread %s, log %s): %v", r.Cid(), tid, lid, err)
					break
				}
				prs = append(prs, pr)
			}

			if len(prs) == 0 {
				// do not include logs with no records in reply
				return
			}

			mx.Lock()
			pbrecs.Logs = append(pbrecs.Logs, &pb.GetRecordsReply_LogEntry{
				LogID:   &pb.ProtoPeerID{ID: lid},
				Records: prs,
				Log:     pblg,
			})
			mx.Unlock()

			log.Debugf("sending %d records in log %s to %s", len(recs), lid, pid)
		}(req.Body.ThreadID.ID, lg.ID, offset, limit)
	}

	wg.Wait()
	return pbrecs, nil
}

// PushRecord receives a push record request.
func (s *server) PushRecord(ctx context.Context, req *pb.PushRecordRequest) (*pb.PushRecordReply, error) {
	pid, err := peerIDFromContext(ctx)
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
	// TODO: replace with counter check (if possible)
	if knownRecord, err := s.net.isKnown(rec.Cid()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	} else if knownRecord {
		return &pb.PushRecordReply{}, nil
	}

	if err = rec.Verify(logpk); err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	if err = s.net.PutRecord(ctx, req.Body.ThreadID.ID, req.Body.LogID.ID, rec, req.Counter); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.PushRecordReply{}, nil
}

// ExchangeEdges receives an exchange edges request.
func (s *server) ExchangeEdges(ctx context.Context, req *pb.ExchangeEdgesRequest) (*pb.ExchangeEdgesReply, error) {
	pid, err := peerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	log.Debugf("received exchange edges request from %s", pid)

	var reply pb.ExchangeEdgesReply
	for _, entry := range req.Body.Threads {
		var tid = entry.ThreadID.ID
		switch addrsEdgeLocal, headsEdgeLocal, err := s.localEdges(tid); err {
		case errNoAddrsEdge, errNoHeadsEdge, nil:
			var (
				addrsEdgeRemote = entry.AddressEdge
				headsEdgeRemote = entry.HeadsEdge
			)

			// need to get new logs only if we have non empty addresses on remote and the hashes are different
			if addrsEdgeRemote != lstoreds.EmptyEdgeValue && addrsEdgeLocal != addrsEdgeRemote {
				prt := callPriorityLow
				updateLogs := s.net.updateLogsFromPeer
				// if we don't have the thread locally
				if addrsEdgeLocal == lstoreds.EmptyEdgeValue {
					prt = callPriorityHigh // we have to add thread in pubsub, not just update its logs
					updateLogs = func(ctx context.Context, p peer.ID, t thread.ID) error {
						if err := s.net.updateLogsFromPeer(ctx, p, t); err != nil {
							return err
						}
						if s.net.server.ps != nil {
							return s.net.server.ps.Add(t)
						}
						return nil
					}
				}
				if s.net.queueGetLogs.Schedule(pid, tid, prt, updateLogs) {
					log.Debugf("log information update for thread %s from %s scheduled", tid, pid)
				}
			}

			// need to get new records only if we have non empty heads on remote and the hashes are different
			if headsEdgeRemote != lstoreds.EmptyEdgeValue && headsEdgeLocal != headsEdgeRemote {
				if s.net.queueGetRecords.Schedule(pid, tid, callPriorityLow, s.net.updateRecordsFromPeer) {
					log.Debugf("record update for thread %s from %s scheduled", tid, pid)
				}
			}

			// setting "exists" for backwards compatibility with older versions
			// to get exactly same behaviour as was before
			exists := true
			if addrsEdgeLocal == lstoreds.EmptyEdgeValue || headsEdgeLocal == lstoreds.EmptyEdgeValue {
				exists = false
			}

			reply.Edges = append(reply.Edges, &pb.ExchangeEdgesReply_ThreadEdges{
				ThreadID:    &pb.ProtoThreadID{ID: tid},
				Exists:      exists,
				AddressEdge: addrsEdgeLocal,
				HeadsEdge:   headsEdgeLocal,
			})

		default:
			return nil, fmt.Errorf("getting edges for %s: %w", tid, err)
		}
	}

	return &reply, nil
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

// headsChanged determines if thread heads are different from the requested offsets.
func (s *server) headsChanged(req *pb.GetRecordsRequest) (bool, error) {
	var reqHeads = make([]util.LogHead, len(req.Body.Logs))
	for i, l := range req.Body.GetLogs() {
		reqHeads[i] = util.LogHead{
			Head: thread.Head{
				ID:      l.Offset.Cid,
				Counter: l.Counter,
			},
			LogID: l.LogID.ID,
		}
	}
	var currEdge, err = s.net.store.HeadsEdge(req.Body.ThreadID.ID)
	switch {
	case err == nil:
		return util.ComputeHeadsEdge(reqHeads) != currEdge, nil
	case errors.Is(err, lstore.ErrThreadNotFound):
		// no local heads, but there could be missing logs info in reply
		return true, nil
	default:
		return false, err
	}
}

// localEdges returns values of local addresses/heads edges for the thread.
func (s *server) localEdges(tid thread.ID) (addrsEdge, headsEdge uint64, err error) {
	headsEdge = lstoreds.EmptyEdgeValue
	addrsEdge, err = s.net.store.AddrsEdge(tid)
	if err != nil {
		if errors.Is(err, lstore.ErrThreadNotFound) {
			err = errNoAddrsEdge
		} else {
			err = fmt.Errorf("address edge: %w", err)
		}
		return
	}
	headsEdge, err = s.net.store.HeadsEdge(tid)
	if err != nil {
		if errors.Is(err, lstore.ErrThreadNotFound) {
			err = errNoHeadsEdge
		} else {
			err = fmt.Errorf("heads edge: %w", err)
		}
	}
	return
}

// peerIDFromContext returns peer ID from the GRPC context
func peerIDFromContext(ctx context.Context) (peer.ID, error) {
	ctxPeer, ok := grpcpeer.FromContext(ctx)
	if !ok {
		return "", errors.New("unable to identify stream peer")
	}
	pid, err := peer.Decode(ctxPeer.Addr.String())
	if err != nil {
		return "", fmt.Errorf("parsing stream peer id: %v", err)
	}
	return pid, nil
}

// logToProto returns a proto log from a thread log.
func logToProto(l thread.LogInfo) *pb.Log {
	return &pb.Log{
		ID:      &pb.ProtoPeerID{ID: l.ID},
		PubKey:  &pb.ProtoPubKey{PubKey: l.PubKey},
		Addrs:   addrsToProto(l.Addrs),
		Head:    &pb.ProtoCid{Cid: l.Head.ID},
		Counter: l.Head.Counter,
	}
}

// logFromProto returns a thread log from a proto log.
func logFromProto(l *pb.Log) thread.LogInfo {
	return thread.LogInfo{
		ID:     l.ID.ID,
		PubKey: l.PubKey.PubKey,
		Addrs:  addrsFromProto(l.Addrs),
		Head:   thread.Head{
			ID:      l.Head.Cid,
			Counter: l.Counter,
		},
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

func minInt(x, y int) int {
	if x < y {
		return x
	}
	return y
}
