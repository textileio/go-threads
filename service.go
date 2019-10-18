package threads

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/ipfs/go-cid"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	gostream "github.com/libp2p/go-libp2p-gostream"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-core/crypto"
	"github.com/textileio/go-textile-core/crypto/asymmetric"
	"github.com/textileio/go-textile-core/crypto/symmetric"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
	"github.com/textileio/go-textile-threads/cbor"
	pb "github.com/textileio/go-textile-threads/pb"
	"google.golang.org/grpc"
)

const (
	// reqTimeout is the duration to wait for a request to complete.
	reqTimeout = time.Second * 5
)

// service implements the Threads RPC service.
type service struct {
	threads *threads
	pubsub  *pubsub.PubSub
}

// newService creates a new threads network service.
func newService(t *threads) (*service, error) {
	ps, err := pubsub.NewGossipSub(
		t.ctx,
		t.host,
		pubsub.WithMessageSigning(false),
		pubsub.WithStrictSignatureVerification(false))
	if err != nil {
		return nil, err
	}

	s := &service{
		threads: t,
		pubsub:  ps,
	}

	ts, err := t.Threads()
	if err != nil {
		return nil, err
	}
	for _, id := range ts {
		go s.subscribe(id)
	}

	// @todo: ts.pubsub.RegisterTopicValidator()

	return s, nil
}

// Push receives a push request.
func (s *service) Push(ctx context.Context, req *pb.PushRequest) (*pb.PushReply, error) {
	if req.Header == nil {
		return nil, fmt.Errorf("request header is required")
	}

	// Verify the request
	reqpk, err := requestPubKey(req)
	if err != nil {
		return nil, err
	}
	err = verifyRequestSignature(req.Record, reqpk, req.Header.Signature)
	if err != nil {
		return nil, err
	}

	reply := &pb.PushReply{}

	// Unpack the record
	var fkey []byte
	if req.Header.FollowKey != nil {
		fkey = req.Header.FollowKey
	} else {
		fkey, err = s.threads.FollowKey(req.ThreadID.ID, req.LogID.ID)
		if err != nil {
			return nil, err
		}
	}
	if fkey == nil {
		return nil, fmt.Errorf("follow key not found")
	}

	rec, err := recordFromProto(req.Record, fkey)
	if err != nil {
		return nil, err
	}
	knownRecord, err := s.threads.bstore.Has(rec.Cid())
	if err != nil {
		return nil, err
	}
	if knownRecord {
		return reply, nil
	}

	// Check if this log already exists
	logpk, err := s.threads.PubKey(req.ThreadID.ID, req.LogID.ID)
	if err != nil {
		return nil, err
	}
	if logpk == nil {
		var kid peer.ID
		if req.Header.ReadKeyLogID != nil {
			kid = req.Header.ReadKeyLogID.ID
		}
		lg, newAddr, err := s.handleNewLogs(ctx, req.ThreadID.ID, req.LogID.ID, kid, rec)
		if err != nil {
			return nil, err
		}

		if newAddr != nil {
			reply.NewAddr = &pb.ProtoAddr{Multiaddr: newAddr}
		}

		if lg != nil {
			// Notify existing logs of our new log
			logs, err := cbor.NewLogs([]thread.LogInfo{*lg}, true)
			if err != nil {
				return nil, err
			}
			_, err = s.threads.Add(
				ctx,
				logs,
				tserv.AddOpt.ThreadID(req.ThreadID.ID),
				tserv.AddOpt.KeyLog(req.LogID.ID))
			if err != nil {
				return nil, err
			}
		}
	} else {
		// Verify node
		if err = rec.Verify(logpk); err != nil {
			return nil, err
		}

		if err = s.handleLogUpdate(ctx, req.ThreadID.ID, req.LogID.ID, rec); err == nil {
			log.Infof("log %s updated", req.LogID.ID.String())
		}
	}

	if err = s.threads.Put(
		ctx,
		rec,
		tserv.PutOpt.ThreadID(req.ThreadID.ID),
		tserv.PutOpt.LogID(req.LogID.ID)); err != nil {
		return nil, err
	}

	return reply, nil
}

// Pull receives a pull request.
func (s *service) Pull(ctx context.Context, req *pb.PullRequest) (*pb.PullReply, error) {
	recs, err := s.threads.pullLocal(
		ctx, req.ThreadID.ID,
		req.LogID.ID,
		req.Offset.Cid,
		int(req.Limit))
	if err != nil {
		return nil, err
	}

	pbrecs := &pb.PullReply{
		Records: make([]*pb.Record, len(recs)),
	}
	for i, r := range recs {
		pbrecs.Records[i], err = cbor.RecordToProto(ctx, s.threads.dagService, r)
		if err != nil {
			return nil, err
		}
	}
	return pbrecs, nil
}

// push a record to log addresses and thread topic.
func (s *service) push(
	ctx context.Context,
	rec thread.Record,
	id thread.ID,
	lid peer.ID,
	settings *tserv.AddSettings,
) error {
	var addrs []ma.Multiaddr
	// Collect known writers
	info, err := s.threads.ThreadInfo(settings.ThreadID)
	if err != nil {
		return err
	}
	for _, l := range info.Logs {
		if l.String() == lid.String() {
			continue
		}
		laddrs, err := s.threads.Addrs(settings.ThreadID, l)
		if err != nil {
			return err
		}
		addrs = append(addrs, laddrs...)
	}

	// Add additional addresses
	addrs = append(addrs, settings.Addrs...)

	// Serialize and sign the record for transport
	pbrec, err := cbor.RecordToProto(ctx, s.threads.dagService, rec)
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

	var keyLog *pb.ProtoPeerID
	logKey, err := s.threads.ReadKey(settings.ThreadID, settings.KeyLog)
	if err != nil {
		return err
	}
	if logKey != nil {
		keyLog = &pb.ProtoPeerID{ID: settings.KeyLog}
	}
	followKey, err := s.threads.FollowKey(id, lid)
	if err != nil {
		return err
	}
	req := &pb.PushRequest{
		Header: &pb.PushRequest_Header{
			From:         &pb.ProtoPeerID{ID: s.threads.host.ID()},
			Signature:    sig,
			Key:          &pb.ProtoPubKey{PubKey: sk.GetPublic()},
			FollowKey:    followKey,
			ReadKeyLogID: keyLog,
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

			log.Debugf("pushing record to %s...", p)

			cctx, cancel := context.WithTimeout(ctx, reqTimeout)
			defer cancel()
			conn, err := s.dial(cctx, pid, grpc.WithInsecure())
			if err != nil {
				log.Errorf("dial %s failed: %s", p, err)
				return
			}
			client := pb.NewThreadsClient(conn)
			reply, err := client.Push(cctx, req)
			if err != nil {
				log.Error(err)
				return
			}

			log.Debugf("received reply from %s", p)

			// Handle new log addresses
			if reply.NewAddr != nil {
				err = s.threads.AddAddr(id, lid, reply.NewAddr.Multiaddr, pstore.PermanentAddrTTL)
				if err != nil {
					log.Error(err)
					return
				}

				// Notify others
				go func() {
					pk, err := s.threads.PubKey(id, lid)
					if err != nil {
						log.Error(err)
						return
					}
					lg := thread.LogInfo{
						ID:     lid,
						PubKey: pk,
						Addrs:  []ma.Multiaddr{reply.NewAddr.Multiaddr},
					}
					logs, err := cbor.NewLogs([]thread.LogInfo{lg}, true)
					if err != nil {
						log.Error(err)
						return
					}
					_, err = s.threads.Add(
						ctx,
						logs,
						tserv.AddOpt.ThreadID(req.ThreadID.ID),
						tserv.AddOpt.KeyLog(req.LogID.ID))
					if err != nil {
						log.Error(err)
						return
					}
				}()
			}
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

// pull records from log addresses.
func (s *service) pull(
	ctx context.Context,
	id thread.ID,
	lid peer.ID,
	offset cid.Cid,
	limit int,
) ([]thread.Record, error) {
	lg, err := s.threads.LogInfo(id, lid)
	if err != nil {
		return nil, err
	}
	if lg.PubKey == nil {
		return nil, fmt.Errorf("log not found")
	}
	fk, err := crypto.ParseDecryptionKey(lg.FollowKey)
	if err != nil {
		return nil, err
	}

	req := &pb.PullRequest{
		Header: &pb.PullRequest_Header{
			From: &pb.ProtoPeerID{ID: s.threads.host.ID()},
		},
		ThreadID: &pb.ProtoThreadID{ID: id},
		LogID:    &pb.ProtoPeerID{ID: lid},
		Offset:   &pb.ProtoCid{Cid: offset},
		Limit:    int32(limit),
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
			pid, err := peer.IDB58Decode(p)
			if err != nil {
				log.Error(err)
				return
			}
			if pid.String() == s.threads.host.ID().String() {
				return
			}

			log.Debugf("pulling records from %s...", p)

			cctx, cancel := context.WithTimeout(ctx, reqTimeout)
			defer cancel()
			conn, err := s.dial(cctx, pid, grpc.WithInsecure())
			if err != nil {
				log.Errorf("dial %s failed: %s", p, err)
				return
			}
			client := pb.NewThreadsClient(conn)
			reply, err := client.Pull(cctx, req)
			if err != nil {
				log.Error(err)
				return
			}

			for _, r := range reply.Records {
				rec, err := cbor.RecordFromProto(r, fk)
				if err != nil {
					log.Error(err)
					return
				}
				recs.Store(rec.Cid(), rec)
			}

			log.Debugf("received reply from %s", p)
		}(addr)
	}

	wg.Wait()
	return recs.List(), nil
}

// pullHistory downloads a logs entire history.
// @todo: Offset needs to be expanded into a start and stop cid
func (s *service) pullHistory(ctx context.Context, id thread.ID, lid peer.ID) error {
	recs, err := s.pull(ctx, id, lid, cid.Undef, MaxPullLimit)
	if err != nil {
		return err
	}
	for _, r := range recs {
		err = s.threads.Put(ctx, r, tserv.PutOpt.ThreadID(id), tserv.PutOpt.LogID(lid))
		if err != nil {
			return err
		}
	}
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
		c, err := gostream.Dial(ctx, s.threads.host, id, IPELProtocol)
		return c, err
	})
}

// subscribe to a thread for updates.
func (s *service) subscribe(id thread.ID) {
	sub, err := s.pubsub.Subscribe(id.String())
	if err != nil {
		log.Error(err)
		return
	}

	for {
		msg, err := sub.Next(s.threads.ctx)
		if err != nil {
			log.Error(err)
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

		req := new(pb.PushRequest)
		err = proto.Unmarshal(msg.Data, req)
		if err != nil {
			log.Error(err)
			continue
		}

		log.Debugf("received multicast request from %s", from.String())

		_, err = s.Push(s.threads.ctx, req)
		if err != nil {
			log.Error(err)
			continue
		}
	}
}

// publish a request to a thread.
func (s *service) publish(id thread.ID, req *pb.PushRequest) error {
	data, err := req.Marshal()
	if err != nil {
		return err
	}
	return s.pubsub.Publish(id.String(), data)
}

// handleNewLogs processes a record as a list of new logs.
func (s *service) handleNewLogs(
	ctx context.Context,
	id thread.ID,
	lid peer.ID,
	kid peer.ID,
	rec thread.Record,
) (ownLog *thread.LogInfo, newAddr ma.Multiaddr, err error) {
	event, err := cbor.EventFromRecord(ctx, s.threads.dagService, rec)
	if err != nil {
		return
	}

	var newThread bool
	var key crypto.DecryptionKey
	ti, err := s.threads.ThreadInfo(id)
	if err != nil {
		return
	}
	if ti.Logs.Len() > 0 {
		// Thread exists—there should be a key log id
		var logKey []byte
		logKey, err = s.threads.ReadKey(id, kid)
		if err != nil {
			return
		}
		if logKey == nil {
			err = fmt.Errorf("read key not found")
			return
		}
		key, err = symmetric.NewKey(logKey)
		if err != nil {
			return
		}
	} else {
		// Thread does not exist—try host peer's key
		sk := s.threads.getPrivKey()
		if sk == nil {
			err = fmt.Errorf("key for host not found")
			return
		}
		key, err = asymmetric.NewDecryptionKey(sk)
		if err != nil {
			return
		}
		newThread = true
	}

	body, err := event.GetBody(ctx, s.threads.dagService, key)
	if err != nil {
		return
	}
	logs, err := cbor.LogsFromNode(body)
	if err != nil {
		return
	}

	// Add incoming logs
	for _, lg := range logs.Logs {
		if !lg.ID.MatchesPublicKey(lg.PubKey) {
			err = fmt.Errorf("invalid log")
			return
		}
		if lg.ID.String() == lid.String() { // This is the log carrying the event
			err = rec.Verify(lg.PubKey)
			if err != nil {
				return
			}
		}
		err = s.threads.AddLog(id, lg)
		if err != nil {
			return
		}

		// Download log history
		// @todo: Should this even happen unless direcly asked for by a user?
		// @todo: If auto, do we need to queue downloads a la, threads v1?
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			if err := s.pullHistory(ctx, id, lg.ID); err != nil {
				log.Errorf("error pulling history: %s", err)
			}
		}()
	}

	// Create an own log if this is a new thread
	if logs.Readable() && newThread {
		var lg thread.LogInfo
		lg, err = s.threads.createLog()
		if err != nil {
			return
		}
		err = s.threads.AddLog(id, lg)
		if err != nil {
			return
		}
		ownLog = &lg
	}

	// If not readable, return a new address for the sender's log.
	// This peer becomes a follower.
	if !logs.Readable() {
		newAddr, err = ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", s.threads.host.ID().String()))
		if err != nil {
			return
		}
		err = s.threads.AddAddr(id, lid, newAddr, pstore.PermanentAddrTTL)
		if err != nil {
			return
		}
	}

	// Subscribe to the new thread
	if newThread {
		go s.subscribe(id)
	}
	return
}

// handleLogUpdate processes a record as a log update.
func (s *service) handleLogUpdate(ctx context.Context, id thread.ID, lid peer.ID, rec thread.Record) error {
	event, err := cbor.EventFromRecord(ctx, s.threads.dagService, rec)
	if err != nil {
		return err
	}
	rk, err := s.threads.ReadKey(id, lid)
	if err != nil {
		return err
	}
	if rk == nil {
		return nil // No key, carry on
	}
	key, err := symmetric.NewKey(rk)
	if err != nil {
		return err
	}

	body, err := event.GetBody(ctx, s.threads.dagService, key)
	if err != nil {
		return err
	}
	logs, err := cbor.LogsFromNode(body)
	if err != nil {
		return err
	}

	// Update the record's log
	for _, lg := range logs.Logs {
		if !lg.ID.MatchesPublicKey(lg.PubKey) {
			return fmt.Errorf("invalid log")
		}
		if lg.ID.String() != lid.String() { // We only want updates from the owner
			continue
		}
		err = rec.Verify(lg.PubKey)
		if err != nil {
			return err
		}
		err = s.threads.AddLog(id, lg)
		if err != nil {
			return err
		}
	}
	return nil
}

// requestPubKey returns the key associated with a request.
func requestPubKey(r *pb.PushRequest) (ic.PubKey, error) {
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
func verifyRequestSignature(rec *pb.Record, pk ic.PubKey, sig []byte) error {
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

// recordFromProto returns a thread record from a proto record.
func recordFromProto(rec *pb.Record, key []byte) (thread.Record, error) {
	dkey, err := crypto.ParseDecryptionKey(key)
	if err != nil {
		return nil, err
	}
	return cbor.RecordFromProto(rec, dkey)
}
