package threads

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	gostream "github.com/libp2p/go-libp2p-gostream"
	"github.com/libp2p/go-libp2p-pubsub"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-core/crypto"
	"github.com/textileio/go-textile-core/crypto/asymmetric"
	pbc "github.com/textileio/go-textile-core/crypto/pb"
	"github.com/textileio/go-textile-core/crypto/symmetric"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
	"github.com/textileio/go-textile-threads/cbor"
	pb "github.com/textileio/go-textile-threads/pb"
	"github.com/textileio/go-textile-threads/util"
	"google.golang.org/grpc"
)

const (
	// pushTimeout is the duration to wait for a push to succeed.
	pushTimeout = time.Second * 5
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

	for _, id := range t.Threads() {
		go s.subscribe(id)
	}

	// @todo: ts.pubsub.RegisterTopicValidator()

	return s, nil
}

// Push receives a record request.
func (s *service) Push(ctx context.Context, req *pb.RecordRequest) (*pb.RecordReply, error) {
	// Verify the request
	reqpk, err := requestPubKey(req)
	if err != nil {
		return nil, err
	}
	err = verifyRequestSignature(req.Record, reqpk, req.Header.Signature)
	if err != nil {
		return nil, err
	}

	reply := &pb.RecordReply{Ok: true}

	// Unpack the record
	var fkey []byte
	if req.Header.FollowKey != nil {
		fkey = req.Header.FollowKey
	} else {
		fkey = s.threads.FollowKey(req.ThreadId.ID, req.LogId.ID)
	}
	if fkey == nil {
		return nil, fmt.Errorf("could not find follow key")
	}

	rec, err := recordFromProto(req.Record, fkey)
	if err != nil {
		return nil, err
	}
	x, err := s.threads.blocks.Blockstore().Has(rec.Cid())
	if err != nil {
		return nil, err
	}
	if x {
		return reply, nil
	}

	// Check if this log already exists
	logpk := s.threads.PubKey(req.ThreadId.ID, req.LogId.ID)
	if logpk == nil {
		var kid peer.ID
		if req.Header.ReadKeyLogId != nil {
			kid = req.Header.ReadKeyLogId.ID
		}
		log, err := s.handleInvite(ctx, req.ThreadId.ID, req.LogId.ID, kid, rec)
		if err != nil {
			return nil, err
		}

		if log != nil {
			// Notify existing logs of our new log
			invite, err := cbor.NewInvite([]thread.LogInfo{*log}, true)
			if err != nil {
				return nil, err
			}
			_, _, err = s.threads.Add(
				ctx,
				invite,
				tserv.AddOpt.Thread(req.ThreadId.ID),
				tserv.AddOpt.KeyLog(req.LogId.ID))
			if err != nil {
				return nil, err
			}
		}
	} else {
		// Verify node
		err = rec.Verify(logpk)
		if err != nil {
			return nil, err
		}
	}

	err = s.threads.Put(ctx, rec, tserv.PutOpt.Thread(req.ThreadId.ID), tserv.PutOpt.Log(req.LogId.ID))
	if err != nil {
		return nil, err
	}

	return reply, nil
}

// pushAddrs pushes a record to the given addresses.
func (s *service) pushAddrs(ctx context.Context, rec thread.Record, id thread.ID, lid peer.ID, settings *tserv.AddSettings) error {
	var addrs []ma.Multiaddr
	// Collect known writers
	for _, l := range s.threads.ThreadInfo(settings.Thread).Logs {
		if l.String() == lid.String() {
			continue
		}
		addrs = append(addrs, s.threads.Addrs(settings.Thread, l)...)
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
		return fmt.Errorf("could not find key for host")
	}
	sig, err := sk.Sign(payload)
	if err != nil {
		return err
	}

	pk, err := pbc.PublicKeyToProto(sk.GetPublic())
	if err != nil {
		return err
	}
	var keyLog *pb.ProtoPeerID
	logKey := s.threads.ReadKey(settings.Thread, settings.KeyLog)
	if logKey != nil {
		keyLog = &pb.ProtoPeerID{ID: settings.KeyLog}
	}
	req := &pb.RecordRequest{
		Header: &pb.RecordRequest_Header{
			From:         &pb.ProtoPeerID{ID: s.threads.host.ID()},
			Signature:    sig,
			Key:          pk,
			FollowKey:    s.threads.FollowKey(id, lid),
			ReadKeyLogId: keyLog,
		},
		ThreadId: &pb.ProtoThreadID{ID: id},
		LogId:    &pb.ProtoPeerID{ID: lid},
		Record:   pbrec,
	}

	// Push to each address
	wg := sync.WaitGroup{}
	for _, addr := range addrs {
		wg.Add(1)
		go func(addr ma.Multiaddr) {
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

			log.Debugf("pushing request to %s...", p)

			cctx, _ := context.WithTimeout(ctx, pushTimeout)
			conn, err := s.dial(cctx, pid, grpc.WithInsecure())
			if err != nil {
				log.Error(err)
				return
			}
			client := pb.NewThreadsClient(conn)
			reply, err := client.Push(cctx, req)
			if err != nil {
				log.Error(err)
				return
			}

			log.Debugf("reply from %s: %s", p, reply.String())

			wg.Done()
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
func (s *service) dial(ctx context.Context, peerID peer.ID, dialOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	opts := append([]grpc.DialOption{s.getDialOption()}, dialOpts...)
	return grpc.DialContext(ctx, peerID.Pretty(), opts...)
}

// getDialOption returns the WithDialer option to dial via libp2p.
func (s *service) getDialOption() grpc.DialOption {
	return grpc.WithContextDialer(func(ctx context.Context, peerIdStr string) (net.Conn, error) {
		id, err := peer.IDB58Decode(peerIdStr)
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

		req := new(pb.RecordRequest)
		err = proto.Unmarshal(msg.Data, req)
		if err != nil {
			log.Error(err)
			continue
		}

		reply, err := s.Push(s.threads.ctx, req)
		if err != nil {
			log.Error(err)
			continue
		}
		log.Debugf("received multicast request (reply: %s)", reply.String())
	}
}

// publish a request to a thread.
func (s *service) publish(id thread.ID, req *pb.RecordRequest) error {
	data, err := req.Marshal()
	if err != nil {
		return err
	}
	return s.pubsub.Publish(id.String(), data)
}

// handleInvite attempts to process a log record as an invite event.
func (s *service) handleInvite(ctx context.Context, id thread.ID, lid peer.ID, kid peer.ID, rec thread.Record) (*thread.LogInfo, error) {
	event, err := cbor.EventFromRecord(ctx, s.threads.dagService, rec)
	if err != nil {
		return nil, err
	}

	var newThread bool
	var key crypto.DecryptionKey
	if s.threads.ThreadInfo(id).Logs.Len() > 0 {
		// Thread exists—there should be a key log id
		logKey := s.threads.ReadKey(id, kid)
		if logKey == nil {
			return nil, fmt.Errorf("could not find read key")
		}
		key, err = symmetric.NewKey(logKey)
		if err != nil {
			return nil, err
		}
	} else {
		// Thread does not exist—try host peer's key
		sk := s.threads.getPrivKey()
		if sk == nil {
			return nil, fmt.Errorf("could not find key for host")
		}
		key, err = asymmetric.NewDecryptionKey(sk)
		if err != nil {
			return nil, err
		}
		newThread = true
	}

	body, err := event.GetBody(ctx, s.threads.dagService, key)
	if err != nil {
		return nil, err
	}
	logs, reader, err := cbor.InviteFromNode(body)
	if err != nil {
		return nil, err
	}

	// Add incoming logs
	for _, log := range logs {
		if !log.ID.MatchesPublicKey(log.PubKey) {
			return nil, fmt.Errorf("invalid log")
		}
		if log.ID.String() == lid.String() { // This is the log carrying the event
			err = rec.Verify(log.PubKey)
			if err != nil {
				return nil, err
			}
		}
		err = s.threads.AddLog(id, log)
		if err != nil {
			return nil, err
		}
	}

	// Create an own log if this is a new thread
	var ownLog *thread.LogInfo
	if reader && newThread {
		log, err := util.CreateLog(s.threads.host.ID())
		if err != nil {
			return nil, err
		}
		err = s.threads.AddLog(id, log)
		if err != nil {
			return nil, err
		}
		ownLog = &log
	}

	// Subscribe to the new thread
	if newThread {
		go s.subscribe(id)
	}

	return ownLog, nil
}

// requestPubKey returns the key associated with a request.
func requestPubKey(r *pb.RecordRequest) (ic.PubKey, error) {
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
		pubk, err = pbc.PublicKeyFromProto(r.Header.Key)
		if err != nil {
			return nil, fmt.Errorf("cannot unmarshal signing key: %s", err)
		}

		// Verify that the source ID matches the attached key
		if !r.Header.From.ID.MatchesPublicKey(pubk) {
			return nil, fmt.Errorf("bad signing key; source ID %s doesn't match key", r.Header.From.ID)
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
