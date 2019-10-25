package threads

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gogo/status"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	gostream "github.com/libp2p/go-libp2p-gostream"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
	"github.com/textileio/go-textile-threads/cbor"
	pb "github.com/textileio/go-textile-threads/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	// reqTimeout is the duration to wait for a request to complete.
	reqTimeout = time.Second * 5
)

// getLogs in a thread.
func (s *service) getLogs(ctx context.Context, id thread.ID, pid peer.ID) ([]thread.LogInfo, error) {
	req := &pb.GetLogsRequest{
		Header: &pb.GetLogsRequest_Header{
			From: &pb.ProtoPeerID{ID: s.threads.host.ID()},
		},
		ThreadID: &pb.ProtoThreadID{ID: id},
	}

	log.Debugf("getting thread %s logs from %s...", id.String(), pid.String())

	cctx, cancel := context.WithTimeout(ctx, reqTimeout)
	defer cancel()
	conn, err := s.dial(cctx, pid, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	client := pb.NewThreadsClient(conn)
	reply, err := client.GetLogs(cctx, req)
	if err != nil {
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
func (s *service) pushLog(ctx context.Context, id thread.ID, lid peer.ID, pid peer.ID) error {
	lg, err := s.threads.store.LogInfo(id, lid)
	if err != nil {
		return err
	}

	lreq := &pb.PushLogRequest{
		Header: &pb.PushLogRequest_Header{
			From: &pb.ProtoPeerID{ID: s.threads.host.ID()},
		},
		ThreadID: &pb.ProtoThreadID{ID: id},
		Log:      logToProto(lg),
	}

	log.Debugf("pushing log %s to %s...", lg.ID.String(), pid.String())

	cctx, cancel := context.WithTimeout(ctx, reqTimeout)
	defer cancel()
	conn, err := s.dial(cctx, pid, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("dial %s failed: %s", pid.String(), err)
	}
	client := pb.NewThreadsClient(conn)
	_, err = client.PushLog(cctx, lreq)
	return err
}

// records maintains an ordered list of records from multiple sources.
type records struct {
	sync.RWMutex
	m map[cid.Cid]thread.Record
	s []thread.Record
}

// newRecords creates an instance of records.
func newRecords() *records {
	return &records{
		m: make(map[cid.Cid]thread.Record),
		s: make([]thread.Record, 0),
	}
}

// List all records.
func (r *records) List() []thread.Record {
	r.RLock()
	defer r.RUnlock()
	return r.s
}

// Store a record.
func (r *records) Store(key cid.Cid, value thread.Record) {
	r.Lock()
	defer r.Unlock()
	if _, ok := r.m[key]; ok {
		return
	}
	r.m[key] = value
	r.s = append(r.s, value)
}

// getRecords from log addresses.
func (s *service) getRecords(
	ctx context.Context,
	id thread.ID,
	lid peer.ID,
	offsets map[peer.ID]cid.Cid,
	limit int,
) (map[peer.ID][]thread.Record, error) {
	lgs := make(map[peer.ID]thread.LogInfo)
	var err error
	for lid := range offsets {
		lgs[lid], err = s.threads.store.LogInfo(id, lid)
		if err != nil {
			return nil, err
		}
	}

	pblgs := make([]*pb.GetRecordsRequest_LogEntry, 0)
	for _, lg := range lgs {
		pblgs = append(pblgs, &pb.GetRecordsRequest_LogEntry{
			LogID:  &pb.ProtoPeerID{ID: lg.ID},
			Offset: &pb.ProtoCid{Cid: offsets[lg.ID]},
			Limit:  int32(limit),
		})
	}

	req := &pb.GetRecordsRequest{
		Header: &pb.GetRecordsRequest_Header{
			From: &pb.ProtoPeerID{ID: s.threads.host.ID()},
		},
		ThreadID: &pb.ProtoThreadID{ID: id},
		Logs:     pblgs,
	}

	lg, err := s.threads.store.LogInfo(id, lid)
	if err != nil {
		return nil, err
	}
	if lg.PubKey == nil {
		return nil, fmt.Errorf("log not found")
	}

	// Pull from each address
	recs := make(map[peer.ID]*records)
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
			pid, err := peer.IDB58Decode(p)
			if err != nil {
				log.Error(err)
				return
			}
			if pid.String() == s.threads.host.ID().String() {
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
			client := pb.NewThreadsClient(conn)
			reply, err := client.GetRecords(cctx, req)
			if err != nil {
				log.Error(err)
				return
			}

			for _, l := range reply.Logs {
				log.Debugf("received %d records in log %s from %s", len(l.Records), l.LogID.ID.String(), p)

				lg, err := s.threads.store.LogInfo(id, l.LogID.ID)
				if err != nil {
					log.Error(err)
					return
				}
				if lg.PubKey == nil {
					if l.Log != nil {
						lg = logFromProto(l.Log)
						if err = s.threads.store.AddLog(id, lg); err != nil {
							log.Error(err)
							return
						}
					} else {
						continue
					}
				}

				if recs[lg.ID] == nil {
					recs[lg.ID] = newRecords()
				}

				for _, r := range l.Records {
					rec, err := cbor.RecordFromProto(r, lg.FollowKey)
					if err != nil {
						log.Error(err)
						return
					}
					recs[lg.ID].Store(rec.Cid(), rec)
				}
			}
		}(addr)
	}

	wg.Wait()

	res := make(map[peer.ID][]thread.Record)
	for lid, rs := range recs {
		res[lid] = rs.List()
	}
	return res, nil
}

// pushRecord to log addresses and thread topic.
func (s *service) pushRecord(
	ctx context.Context,
	rec thread.Record,
	id thread.ID,
	lid peer.ID,
	settings *tserv.AddSettings,
) error {
	// Collect known writers
	var addrs []ma.Multiaddr
	info, err := s.threads.store.ThreadInfo(settings.ThreadID)
	if err != nil {
		return err
	}
	for _, l := range info.Logs {
		laddrs, err := s.threads.store.Addrs(settings.ThreadID, l)
		if err != nil {
			return err
		}
		addrs = append(addrs, laddrs...)
	}

	// Add additional addresses
	addrs = append(addrs, settings.Addrs...)

	// Serialize and sign the record for transport
	pbrec, err := cbor.RecordToProto(ctx, s.threads, rec)
	if err != nil {
		return err
	}
	payload, err := pbrec.Marshal()
	if err != nil {
		return err
	}
	sk := s.threads.getPrivKey()
	if sk == nil {
		return fmt.Errorf("key for host not found")
	}
	sig, err := sk.Sign(payload)
	if err != nil {
		return err
	}

	req := &pb.PushRecordRequest{
		Header: &pb.PushRecordRequest_Header{
			From:      &pb.ProtoPeerID{ID: s.threads.host.ID()},
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
			pid, err := peer.IDB58Decode(p)
			if err != nil {
				log.Error(err)
				return
			}
			if pid.String() == s.threads.host.ID().String() {
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
			client := pb.NewThreadsClient(conn)
			if _, err = client.PushRecord(cctx, req); err != nil {
				if status.Convert(err).Code() == codes.NotFound {
					log.Debugf("pushing log %s to %s...", lid.String(), p)

					// Send the missing log
					l, err := s.threads.store.LogInfo(id, lid)
					if err != nil {
						log.Error(err)
						return
					}
					lreq := &pb.PushLogRequest{
						Header: &pb.PushLogRequest_Header{
							From: &pb.ProtoPeerID{ID: s.threads.host.ID()},
						},
						ThreadID: &pb.ProtoThreadID{ID: id},
						Log:      logToProto(l),
					}
					if _, err = client.PushLog(cctx, lreq); err != nil {
						log.Error(err)
						return
					}
					return
				}
				log.Error(err)
				return
			}

			log.Debugf("received reply from %s", p)
		}(addr)
	}

	// Finally, publish to the thread's topic
	err = s.publish(id, req)
	if err != nil {
		log.Error(err)
	}

	wg.Wait()
	return nil
}

// dial attempts to open a GRPC connection over libp2p to a peer.
func (s *service) dial(
	ctx context.Context,
	peerID peer.ID,
	dialOpts ...grpc.DialOption,
) (*grpc.ClientConn, error) {
	opts := append([]grpc.DialOption{s.getDialOption()}, dialOpts...)
	return grpc.DialContext(ctx, peerID.Pretty(), opts...)
}

// getDialOption returns the WithDialer option to dial via libp2p.
func (s *service) getDialOption() grpc.DialOption {
	return grpc.WithContextDialer(func(ctx context.Context, peerIDStr string) (net.Conn, error) {
		id, err := peer.IDB58Decode(peerIDStr)
		if err != nil {
			return nil, fmt.Errorf("grpc tried to dial non peer-id: %s", err)
		}
		c, err := gostream.Dial(ctx, s.threads.host, id, ThreadProtocol)
		return c, err
	})
}

// publish a request to a thread.
func (s *service) publish(id thread.ID, req *pb.PushRecordRequest) error {
	data, err := req.Marshal()
	if err != nil {
		return err
	}
	return s.pubsub.Publish(id.String(), data)
}
