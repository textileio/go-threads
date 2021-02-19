package net

import (
	"context"
	"errors"
	"fmt"
	nnet "net"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/status"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	gostream "github.com/libp2p/go-libp2p-gostream"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/cbor"
	core "github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	sym "github.com/textileio/go-threads/crypto/symmetric"
	pb "github.com/textileio/go-threads/net/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
)

var (
	// DialTimeout is the max time duration to wait when dialing a peer.
	DialTimeout = time.Second * 10
	PushTimeout = time.Second * 10
	PullTimeout = time.Second * 10
)

// getLogs in a thread.
func (s *server) getLogs(ctx context.Context, id thread.ID, pid peer.ID) ([]thread.LogInfo, error) {
	sk, err := s.net.store.ServiceKey(id)
	if err != nil {
		return nil, err
	}
	if sk == nil {
		return nil, fmt.Errorf("a service-key is required to request logs")
	}

	body := &pb.GetLogsRequest_Body{
		ThreadID:   &pb.ProtoThreadID{ID: id},
		ServiceKey: &pb.ProtoKey{Key: sk},
	}
	sig, key, err := s.signRequestBody(body)
	if err != nil {
		return nil, err
	}
	req := &pb.GetLogsRequest{
		Header: &pb.Header{
			PubKey:    &pb.ProtoPubKey{PubKey: key},
			Signature: sig,
		},
		Body: body,
	}

	log.Debugf("getting %s logs from %s...", id, pid)

	client, err := s.dial(pid)
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(ctx, PullTimeout)
	defer cancel()
	reply, err := client.GetLogs(cctx, req)
	if err != nil {
		log.Warnf("get logs from %s failed: %s", pid, err)
		return nil, err
	}

	log.Debugf("received %d logs from %s", len(reply.Logs), pid)

	lgs := make([]thread.LogInfo, len(reply.Logs))
	for i, l := range reply.Logs {
		lgs[i] = logFromProto(l)
	}

	return lgs, nil
}

// pushLog to a peer.
func (s *server) pushLog(ctx context.Context, id thread.ID, lg thread.LogInfo, pid peer.ID, sk *sym.Key, rk *sym.Key) error {
	body := &pb.PushLogRequest_Body{
		ThreadID: &pb.ProtoThreadID{ID: id},
		Log:      logToProto(lg),
	}
	if sk != nil {
		body.ServiceKey = &pb.ProtoKey{Key: sk}
	}
	if rk != nil {
		body.ReadKey = &pb.ProtoKey{Key: rk}
	}
	sig, key, err := s.signRequestBody(body)
	if err != nil {
		return err
	}
	lreq := &pb.PushLogRequest{
		Header: &pb.Header{
			PubKey:    &pb.ProtoPubKey{PubKey: key},
			Signature: sig,
		},
		Body: body,
	}

	log.Debugf("pushing log %s to %s...", lg.ID, pid)

	client, err := s.dial(pid)
	if err != nil {
		return fmt.Errorf("dial %s failed: %w", pid, err)
	}
	cctx, cancel := context.WithTimeout(ctx, PushTimeout)
	defer cancel()
	_, err = client.PushLog(cctx, lreq)
	if err != nil {
		return fmt.Errorf("push log to %s failed: %w", pid, err)
	}
	return err
}

// getRecords from specified peers.
func (s *server) getRecords(
	peers []peer.ID,
	tid thread.ID,
	offsets map[peer.ID]cid.Cid,
	limit int,
) (map[peer.ID][]core.Record, error) {
	req, sk, err := s.buildGetRecordsRequest(tid, offsets, limit)
	if err != nil {
		return nil, err
	}

	var (
		rc = newRecordCollector()
		wg sync.WaitGroup
	)

	// Pull from every peer
	for _, p := range peers {
		wg.Add(1)

		go withErrLog(p, func(pid peer.ID) error {
			defer wg.Done()

			return s.net.queueGetRecords.Call(pid, tid, func(ctx context.Context, pid peer.ID, tid thread.ID) error {
				recs, err := s.getRecordsFromPeer(ctx, tid, pid, req, sk)
				if err != nil {
					return err
				}
				for lid, rs := range recs {
					for _, rec := range rs {
						rc.Store(lid, rec)
					}
				}
				return nil
			})
		})
	}
	wg.Wait()

	return rc.List()
}

func (s *server) buildGetRecordsRequest(
	tid thread.ID,
	offsets map[peer.ID]cid.Cid,
	limit int,
) (req *pb.GetRecordsRequest, serviceKey *sym.Key, err error) {
	serviceKey, err = s.net.store.ServiceKey(tid)
	if err != nil {
		err = fmt.Errorf("obtaining service key: %w", err)
		return
	} else if serviceKey == nil {
		err = errors.New("a service-key is required to request records")
		return
	}

	var pblgs = make([]*pb.GetRecordsRequest_Body_LogEntry, 0, len(offsets))
	for lid, offset := range offsets {
		pblgs = append(pblgs, &pb.GetRecordsRequest_Body_LogEntry{
			LogID:  &pb.ProtoPeerID{ID: lid},
			Offset: &pb.ProtoCid{Cid: offset},
			Limit:  int32(limit),
		})
	}

	body := &pb.GetRecordsRequest_Body{
		ThreadID:   &pb.ProtoThreadID{ID: tid},
		ServiceKey: &pb.ProtoKey{Key: serviceKey},
		Logs:       pblgs,
	}

	sig, key, err := s.signRequestBody(body)
	if err != nil {
		err = fmt.Errorf("signing GetRecords request: %w", err)
		return
	}

	req = &pb.GetRecordsRequest{
		Header: &pb.Header{
			PubKey:    &pb.ProtoPubKey{PubKey: key},
			Signature: sig,
		},
		Body: body,
	}
	return
}

// Send GetRecords request to a certain peer.
func (s *server) getRecordsFromPeer(
	ctx context.Context,
	tid thread.ID,
	pid peer.ID,
	req *pb.GetRecordsRequest,
	serviceKey *sym.Key,
) (map[peer.ID][]core.Record, error) {
	log.Debugf("getting records from %s...", pid)
	client, err := s.dial(pid)
	if err != nil {
		return nil, fmt.Errorf("dial %s failed: %w", pid, err)
	}

	recs := make(map[peer.ID][]core.Record)
	cctx, cancel := context.WithTimeout(ctx, PullTimeout)
	defer cancel()
	reply, err := client.GetRecords(cctx, req)
	if err != nil {
		log.Warnf("get records from %s failed: %s", pid, err)
		return recs, nil
	}

	for _, l := range reply.Logs {
		var logID = l.LogID.ID
		log.Debugf("received %d records in log %s from %s", len(l.Records), logID, pid)

		if l.Log != nil && len(l.Log.Addrs) > 0 {
			if err = s.net.store.AddAddrs(tid, logID, addrsFromProto(l.Log.Addrs), pstore.PermanentAddrTTL); err != nil {
				return nil, err
			}
		}

		pk, err := s.net.store.PubKey(tid, logID)
		if err != nil {
			return nil, err
		}

		if pk == nil {
			if l.Log == nil || l.Log.PubKey == nil {
				// cannot verify received records
				continue
			}
			if err := s.net.store.AddPubKey(tid, logID, l.Log.PubKey); err != nil {
				return nil, err
			}
			pk = l.Log.PubKey
		}

		for _, r := range l.Records {
			rec, err := cbor.RecordFromProto(r, serviceKey)
			if err != nil {
				return nil, err
			}
			if err = rec.Verify(pk); err != nil {
				return nil, err
			}
			recs[logID] = append(recs[logID], rec)
		}
	}

	return recs, nil
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
	peers, err := s.net.uniquePeers(addrs)
	if err != nil {
		return err
	}

	pbrec, err := cbor.RecordToProto(ctx, s.net, rec)
	if err != nil {
		return err
	}
	body := &pb.PushRecordRequest_Body{
		ThreadID: &pb.ProtoThreadID{ID: id},
		LogID:    &pb.ProtoPeerID{ID: lid},
		Record:   pbrec,
	}
	sig, key, err := s.signRequestBody(body)
	if err != nil {
		return err
	}
	req := &pb.PushRecordRequest{
		Header: &pb.Header{
			PubKey:    &pb.ProtoPubKey{PubKey: key},
			Signature: sig,
		},
		Body: body,
	}

	// Push to each address
	for _, p := range peers {
		go withErrLog(p, func(pid peer.ID) error {
			client, err := s.dial(pid)
			if err != nil {
				return fmt.Errorf("dial %s failed: %w", pid, err)
			}
			cctx, cancel := context.WithTimeout(context.Background(), PushTimeout)
			defer cancel()
			if _, err = client.PushRecord(cctx, req); err != nil {
				if status.Convert(err).Code() == codes.NotFound { // Send the missing log
					log.Debugf("pushing log %s to %s...", lid, pid)
					l, err := s.net.store.GetLog(id, lid)
					if err != nil {
						return err
					}
					body := &pb.PushLogRequest_Body{
						ThreadID: &pb.ProtoThreadID{ID: id},
						Log:      logToProto(l),
					}
					sig, key, err := s.signRequestBody(body)
					if err != nil {
						return err
					}
					lreq := &pb.PushLogRequest{
						Header: &pb.Header{
							PubKey:    &pb.ProtoPubKey{PubKey: key},
							Signature: sig,
						},
						Body: body,
					}
					if _, err = client.PushLog(cctx, lreq); err != nil {
						log.Warnf("push log to %s failed: %s", pid, err)
						return nil
					}
					return nil
				}
				log.Warnf("push record to %s failed: %s", pid, err)
				return nil
			}
			return nil
		})
	}

	// Finally, publish to the thread's topic
	if s.ps != nil {
		if err = s.ps.Publish(ctx, id, req); err != nil {
			log.Errorf("error publishing record: %s", err)
		}
	}

	return nil
}

// exchangeEdges of specified threads with a peer.
func (s *server) exchangeEdges(
	ctx context.Context,
	pid peer.ID,
	tids []thread.ID,
) error {
	// TODO implement
	return nil
}

// dial attempts to open a gRPC connection over libp2p to a peer.
func (s *server) dial(peerID peer.ID) (pb.ServiceClient, error) {
	s.Lock()
	defer s.Unlock()
	conn, ok := s.conns[peerID]
	if ok {
		if conn.GetState() == connectivity.Shutdown {
			if err := conn.Close(); err != nil {
				log.Errorf("error closing connection: %v", err)
			}
		} else {
			return pb.NewServiceClient(conn), nil
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), DialTimeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx, peerID.Pretty(), s.opts...)
	if err != nil {
		return nil, err
	}
	s.conns[peerID] = conn
	return pb.NewServiceClient(conn), nil
}

// getLibp2pDialer returns a WithContextDialer option for libp2p dialing.
func (s *server) getLibp2pDialer() grpc.DialOption {
	return grpc.WithContextDialer(func(ctx context.Context, peerIDStr string) (nnet.Conn, error) {
		id, err := peer.Decode(peerIDStr)
		if err != nil {
			return nil, fmt.Errorf("grpc tried to dial non peerID: %w", err)
		}

		conn, err := gostream.Dial(ctx, s.net.host, id, thread.Protocol)
		if err != nil {
			return nil, fmt.Errorf("gostream dial failed: %w", err)
		}

		return conn, nil
	})
}

// signRequestBody signs an outbound request body with the hosts's private key.
func (s *server) signRequestBody(msg proto.Marshaler) (sig []byte, pk crypto.PubKey, err error) {
	payload, err := msg.Marshal()
	if err != nil {
		return
	}
	sk := s.net.getPrivKey()
	if sk == nil {
		err = fmt.Errorf("private key for host not found")
		return
	}
	sig, err = sk.Sign(payload)
	if err != nil {
		return
	}
	return sig, sk.GetPublic(), nil
}

func withErrLog(pid peer.ID, f func(pid peer.ID) error) {
	if err := f(pid); err != nil {
		log.Error(err.Error())
	}
}
