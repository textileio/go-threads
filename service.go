package threads

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	gostream "github.com/libp2p/go-libp2p-gostream"
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
	pushTimeout = time.Second * 5
)

// service implements the Threads RPC service.
type service struct {
	threads *threads
}

// Echo asks a node to respond with a message.
func (s *service) Push(ctx context.Context, req *pb.RecordRequest) (*pb.RecordReply, error) {
	rpk, err := requestPubKey(req)
	if err != nil {
		return nil, err
	}
	err = verifyRequestSignature(req.Record, rpk, req.Header.Signature)
	if err != nil {
		return nil, err
	}

	var rec thread.Record
	logpk := s.threads.PubKey(req.ThreadId.ID, req.LogId.ID)
	if logpk == nil {
		// Look for a follow key, this might be an invite
		rec, err = recordFromProto(req.Record, req.Header.FollowKey)
		if err != nil {
			return nil, err
		}
		log, err := s.handleInvite(ctx, req.ThreadId.ID, req.LogId.ID, rec)
		if err != nil {
			return nil, err
		}

		// Notify existing logs
		if log != nil {
			invite, err := cbor.NewInvite([]thread.LogInfo{*log}, true)
			if err != nil {
				return nil, err
			}
			key, err := symmetric.NewKey(s.threads.ReadKey(req.ThreadId.ID, req.LogId.ID))
			if err != nil {
				return nil, err
			}
			_, _, err = s.threads.Add(
				ctx,
				invite,
				tserv.AddOpt.Thread(req.ThreadId.ID),
				tserv.AddOpt.Key(key))
			if err != nil {
				return nil, err
			}
		}

	} else {
		rec, err = recordFromProto(req.Record, s.threads.FollowKey(req.ThreadId.ID, req.LogId.ID))
		if err != nil {
			return nil, err
		}

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

	return &pb.RecordReply{
		Ok: true,
	}, nil
}

func (s *service) pushAddrs(ctx context.Context, rec thread.Record, id thread.ID, lid peer.ID, addrs []ma.Multiaddr) error {
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

	req := &pb.RecordRequest{
		Header: &pb.RecordRequest_Header{
			From:      &pb.ProtoPeerID{ID: s.threads.host.ID()},
			Signature: sig,
			Key:       pk,
			FollowKey: s.threads.FollowKey(id, lid),
		},
		ThreadId: &pb.ProtoThreadID{ID: id},
		LogId:    &pb.ProtoPeerID{ID: lid},
		Record:   pbrec,
	}

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
			conn, err := s.dial(cctx, pid, grpc.WithInsecure()) //, grpc.WithBlock())
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

func (s *service) publish(id thread.ID, req *pb.RecordRequest) error {
	data, err := req.Marshal()
	if err != nil {
		return err
	}
	return s.threads.pubsub.Publish(id.String(), data)
}

func (s *service) handleInvite(ctx context.Context, id thread.ID, lid peer.ID, rec thread.Record) (*thread.LogInfo, error) {
	event, err := cbor.EventFromRecord(ctx, s.threads.dagService, rec)
	if err != nil {
		return nil, err
	}
	sk := s.threads.getPrivKey()
	if sk == nil {
		return nil, fmt.Errorf("could not find key for host")
	}
	key, err := asymmetric.NewDecryptionKey(sk)
	if err != nil {
		return nil, err
	}
	nlog := s.threads.getOwnLog(id)
	body, err := event.GetBody(ctx, s.threads.dagService, key)
	if err != nil {
		key, err := symmetric.NewKey(nlog.ReadKey)
		if err != nil {
			return nil, err
		}
		body, err = event.GetBody(ctx, s.threads.dagService, key)
		if err != nil {
			return nil, err
		}
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

	// Create an own log if reader
	if reader && nlog.PubKey == nil {
		nlog, err := util.CreateLog(s.threads.host.ID())
		if err != nil {
			return nil, err
		}
		err = s.threads.AddLog(id, nlog)
		if err != nil {
			return nil, err
		}
		return &nlog, nil
	}

	return nil, nil
}

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

func recordFromProto(rec *pb.Record, key []byte) (thread.Record, error) {
	dkey, err := crypto.ParseDecryptionKey(key)
	if err != nil {
		return nil, err
	}
	return cbor.RecordFromProto(rec, dkey)
}
