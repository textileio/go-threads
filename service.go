package threads

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	gostream "github.com/libp2p/go-libp2p-gostream"
	ma "github.com/multiformats/go-multiaddr"
	pbc "github.com/textileio/go-textile-core/crypto/pb"
	"github.com/textileio/go-textile-core/thread"
	"github.com/textileio/go-textile-threads/cbor"
	pb "github.com/textileio/go-textile-threads/pb"
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
	return &pb.RecordReply{
		Ok: true,
	}, nil
}

func (s *service) pushAddrs(ctx context.Context, rec thread.Record, id thread.ID, lid peer.ID, addrs []ma.Multiaddr) error {
	block, err := rec.GetBlock(ctx, s.threads.dagService)
	if err != nil {
		return err
	}
	event, err := cbor.DecodeEvent(block)
	if err != nil {
		return err
	}
	header, err := event.GetHeader(ctx, s.threads.dagService, nil)
	if err != nil {
		return err
	}
	body, err := event.GetBody(ctx, s.threads.dagService, nil)
	if err != nil {
		return err
	}

	pbrec := &pb.Record{
		Node:       rec.RawData(),
		EventNode:  block.RawData(),
		HeaderNode: header.RawData(),
		BodyNode:   body.RawData(),
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
		Record:    pbrec,
		PublicKey: pk,
		Signature: sig,
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

	wg.Wait()
	return nil

	//switch res.StatusCode {
	//case http.StatusCreated:
	//	fk, err := base64.StdEncoding.DecodeString(res.Header.Get("X-FollowKey"))
	//	if err != nil {
	//		return err
	//	}
	//	followKey, err := crypto.ParseDecryptionKey(fk)
	//	if err != nil {
	//		return err
	//	}
	//	rnode, err := cbor.Unmarshal(body, followKey)
	//	if err != nil {
	//		return err
	//	}
	//	event, err := cbor.GetEventFromNode(ctx, t.dagService, rnode)
	//	if err != nil {
	//		return err
	//	}
	//
	//	rk, err := base64.StdEncoding.DecodeString(res.Header.Get("X-ReadKey"))
	//	if err != nil {
	//		return err
	//	}
	//	readKey, err := crypto.ParseDecryptionKey(rk)
	//	if err != nil {
	//		return err
	//	}
	//	body, err := event.GetBody(ctx, t.dagService, readKey)
	//	if err != nil {
	//		return err
	//	}
	//	logs, _, err := cbor.DecodeInvite(body)
	//	if err != nil {
	//		return err
	//	}
	//	for _, log := range logs {
	//		err = t.AddLog(t, log)
	//		if err != nil {
	//			return err
	//		}
	//	}
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
