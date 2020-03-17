package net

import (
	"context"
	"fmt"
	nnet "net"
	"sync"
	"time"

	"github.com/gogo/status"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	gostream "github.com/libp2p/go-libp2p-gostream"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/cbor"
	core "github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	sym "github.com/textileio/go-threads/crypto/symmetric"
	pb "github.com/textileio/go-threads/net/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	// reqTimeout is the duration to wait for a request to complete.
	reqTimeout = time.Second * 10
)

// getLogs in a thread.
func (s *server) getLogs(ctx context.Context, id thread.ID, pid peer.ID) ([]thread.LogInfo, error) {
	fk, err := s.net.store.FollowKey(id)
	if err != nil {
		return nil, err
	}
	if fk == nil {
		return nil, fmt.Errorf("a follow-key is required to request logs")
	}

	req := &pb.GetLogsRequest{
		Header: &pb.GetLogsRequest_Header{
			From: &pb.ProtoPeerID{ID: s.net.host.ID()},
		},
		ThreadID:  &pb.ProtoThreadID{ID: id},
		FollowKey: &pb.ProtoKey{Key: fk},
	}

	log.Debugf("getting %s logs from %s...", id.String(), pid.String())

	cctx, cancel := context.WithTimeout(ctx, reqTimeout)
	defer cancel()
	conn, err := s.dial(cctx, pid, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	// @todo: Retain connections.
	client := pb.NewServiceClient(conn)
	reply, err := client.GetLogs(cctx, req)
	if err != nil {
		log.Warnf("get logs from %s failed: %s", pid.String(), err)
		return nil, err
	}

	log.Debugf("received %d logs from %s", len(reply.Logs), pid.String())

	lgs := make([]thread.LogInfo, len(reply.Logs))
	for i, l := range reply.Logs {
		lgs[i] = logFromProto(l)
	}

	return lgs, nil
}

// pushLog to a peer.
func (s *server) pushLog(ctx context.Context, id thread.ID, lg thread.LogInfo, pid peer.ID, fk *sym.Key, rk *sym.Key) error {
	lreq := &pb.PushLogRequest{
		Header: &pb.PushLogRequest_Header{
			From: &pb.ProtoPeerID{ID: s.net.host.ID()},
		},
		ThreadID: &pb.ProtoThreadID{ID: id},
		Log:      logToProto(lg),
	}
	if fk != nil {
		lreq.FollowKey = &pb.ProtoKey{Key: fk}
	}
	if rk != nil {
		lreq.ReadKey = &pb.ProtoKey{Key: rk}
	}

	log.Debugf("pushing log %s to %s...", lg.ID.String(), pid.String())

	cctx, cancel := context.WithTimeout(ctx, reqTimeout)
	defer cancel()
	conn, err := s.dial(cctx, pid, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("dial %s failed: %s", pid.String(), err)
	}
	client := pb.NewServiceClient(conn)
	_, err = client.PushLog(cctx, lreq)
	if err != nil {
		return fmt.Errorf("push log to %s failed: %s", pid.String(), err)
	}
	return err
}

// records maintains an ordered list of records from multiple sources.
type records struct {
	sync.RWMutex
	m map[peer.ID]map[cid.Cid]core.Record
	s map[peer.ID][]core.Record
}

// newRecords creates an instance of records.
func newRecords() *records {
	return &records{
		m: make(map[peer.ID]map[cid.Cid]core.Record),
		s: make(map[peer.ID][]core.Record),
	}
}

// List all records.
func (r *records) List() map[peer.ID][]core.Record {
	r.RLock()
	defer r.RUnlock()
	return r.s
}

// Store a record.
func (r *records) Store(p peer.ID, key cid.Cid, value core.Record) {
	r.Lock()
	defer r.Unlock()
	if _, ok := r.m[p]; !ok {
		r.m[p] = make(map[cid.Cid]core.Record)
		r.s[p] = make([]core.Record, 0)
	}
	if _, ok := r.m[p][key]; ok {
		return
	}
	r.m[p][key] = value

	// Sanity check
	if len(r.s[p]) > 0 && r.s[p][len(r.s[p])-1].Cid() != value.PrevID() {
		panic("there is a gap in records list")
	}

	r.s[p] = append(r.s[p], value)
}

// getRecords from log addresses.
func (s *server) getRecords(ctx context.Context, id thread.ID, lid peer.ID, offsets map[peer.ID]cid.Cid, limit int) (map[peer.ID][]core.Record, error) {
	fk, err := s.net.store.FollowKey(id)
	if err != nil {
		return nil, err
	}
	if fk == nil {
		return nil, fmt.Errorf("a follow-key is required to request records")
	}

	pblgs := make([]*pb.GetRecordsRequest_LogEntry, 0, len(offsets))
	for lid, offset := range offsets {
		pblgs = append(pblgs, &pb.GetRecordsRequest_LogEntry{
			LogID:  &pb.ProtoPeerID{ID: lid},
			Offset: &pb.ProtoCid{Cid: offset},
			Limit:  int32(limit),
		})
	}

	req := &pb.GetRecordsRequest{
		Header: &pb.GetRecordsRequest_Header{
			From: &pb.ProtoPeerID{ID: s.net.host.ID()},
		},
		ThreadID:  &pb.ProtoThreadID{ID: id},
		FollowKey: &pb.ProtoKey{Key: fk},
		Logs:      pblgs,
	}

	lg, err := s.net.store.GetLog(id, lid)
	if err != nil {
		return nil, err
	}
	if lg.PubKey == nil {
		return nil, fmt.Errorf("log not found")
	}

	// Pull from each address
	recs := newRecords()
	wg := sync.WaitGroup{}
	for _, addr := range lg.Addrs {
		wg.Add(1)
		go func(addr ma.Multiaddr) {
			defer wg.Done()
			p, err := addr.ValueForProtocol(ma.P_P2P)
			if err != nil {
				log.Error(err)
				return
			}
			pid, err := peer.Decode(p)
			if err != nil {
				log.Error(err)
				return
			}
			if pid.String() == s.net.host.ID().String() {
				return
			}

			log.Debugf("getting records from %s...", p)

			cctx, cancel := context.WithTimeout(ctx, reqTimeout)
			defer cancel()
			conn, err := s.dial(cctx, pid, grpc.WithInsecure())
			if err != nil {
				log.Errorf("dial %s failed: %s", p, err)
				return
			}
			client := pb.NewServiceClient(conn)
			reply, err := client.GetRecords(cctx, req)
			if err != nil {
				log.Warnf("get records from %s failed: %s", p, err)
				return
			}
			for _, l := range reply.Logs {
				log.Debugf("received %d records in log %s from %s", len(l.Records), l.LogID.ID.String(), p)

				lg, err := s.net.store.GetLog(id, l.LogID.ID)
				if err != nil {
					log.Error(err)
					return
				}
				if lg.PubKey == nil {
					if l.Log != nil {
						lg = logFromProto(l.Log)
						lg.Heads = []cid.Cid{}
						if err = s.net.store.AddLog(id, lg); err != nil {
							log.Error(err)
							return
						}
					} else {
						continue
					}
				}

				for _, r := range l.Records {
					rec, err := cbor.RecordFromProto(r, fk)
					if err != nil {
						log.Error(err)
						return
					}
					recs.Store(lg.ID, rec.Cid(), rec)
				}
			}
		}(addr)
	}
	wg.Wait()

	return recs.List(), nil
}

// pushRecord to log addresses and thread topic.
func (s *server) pushRecord(ctx context.Context, id thread.ID, lid peer.ID, rec core.Record) error {
	// Collect known writers
	addrs := make([]ma.Multiaddr, 0)
	info, err := s.net.store.GetThread(id)
	if err != nil {
		return err
	}
	for _, l := range info.Logs {
		addrs = append(addrs, l.Addrs...)
	}

	// Serialize and sign the record for transport
	pbrec, err := cbor.RecordToProto(ctx, s.net, rec)
	if err != nil {
		return err
	}
	payload, err := pbrec.Marshal()
	if err != nil {
		return err
	}
	sk := s.net.getPrivKey()
	if sk == nil {
		return fmt.Errorf("private key for host not found")
	}
	sig, err := sk.Sign(payload)
	if err != nil {
		return err
	}

	req := &pb.PushRecordRequest{
		Header: &pb.PushRecordRequest_Header{
			From:      &pb.ProtoPeerID{ID: s.net.host.ID()},
			Signature: sig,
			Key:       &pb.ProtoPubKey{PubKey: sk.GetPublic()},
		},
		ThreadID: &pb.ProtoThreadID{ID: id},
		LogID:    &pb.ProtoPeerID{ID: lid},
		Record:   pbrec,
	}

	// Push to each address
	wg := sync.WaitGroup{}
	for _, addr := range addrs {
		wg.Add(1)
		go func(addr ma.Multiaddr) {
			defer wg.Done()
			p, err := addr.ValueForProtocol(ma.P_P2P)
			if err != nil {
				log.Error(err)
				return
			}
			pid, err := peer.Decode(p)
			if err != nil {
				log.Error(err)
				return
			}
			if pid.String() == s.net.host.ID().String() {
				return
			}

			log.Debugf("pushing record to %s...", p)

			cctx, cancel := context.WithTimeout(ctx, reqTimeout)
			defer cancel()
			conn, err := s.dial(cctx, pid, grpc.WithInsecure())
			if err != nil {
				log.Errorf("dial %s failed: %s", p, err)
				return
			}
			client := pb.NewServiceClient(conn)
			if _, err = client.PushRecord(cctx, req); err != nil {
				if status.Convert(err).Code() == codes.NotFound {
					log.Debugf("pushing log %s to %s...", lid.String(), p)

					// Send the missing log
					l, err := s.net.store.GetLog(id, lid)
					if err != nil {
						log.Error(err)
						return
					}
					lreq := &pb.PushLogRequest{
						Header: &pb.PushLogRequest_Header{
							From: &pb.ProtoPeerID{ID: s.net.host.ID()},
						},
						ThreadID: &pb.ProtoThreadID{ID: id},
						Log:      logToProto(l),
					}
					if _, err = client.PushLog(cctx, lreq); err != nil {
						log.Warnf("push log to %s failed: %s", p, err)
						return
					}
					return
				}
				log.Warnf("push record to %s failed: %s", p, err)
				return
			}
		}(addr)
	}

	// Finally, publish to the thread's topic
	if err = s.publish(id, req); err != nil {
		log.Error(err)
	}

	wg.Wait()
	return nil
}

// dial attempts to open a GRPC connection over libp2p to a peer.
func (s *server) dial(ctx context.Context, peerID peer.ID, dialOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	opts := append([]grpc.DialOption{s.getDialOption()}, dialOpts...)
	return grpc.DialContext(ctx, peerID.Pretty(), opts...)
}

// getDialOption returns the WithDialer option to dial via libp2p.
func (s *server) getDialOption() grpc.DialOption {
	return grpc.WithContextDialer(func(ctx context.Context, peerIDStr string) (nnet.Conn, error) {
		id, err := peer.Decode(peerIDStr)
		if err != nil {
			return nil, fmt.Errorf("grpc tried to dial non peer-id: %s", err)
		}
		return gostream.Dial(ctx, s.net.host, id, thread.Protocol)
	})
}

// publish a request to a thread.
func (s *server) publish(id thread.ID, req *pb.PushRecordRequest) error {
	data, err := req.Marshal()
	if err != nil {
		return err
	}
	return s.pubsub.Publish(id.String(), data)
}
