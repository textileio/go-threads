package threads

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	pb "github.com/textileio/go-textile-threads/pb"
)

// service implements the Threads RPC service.
type service struct {
	PeerID peer.ID
}

// Echo asks a node to respond with a message.
func (s *service) Push(ctx context.Context, req *pb.RecordRequest) (*pb.RecordReply, error) {
	return &pb.RecordReply{
		Ok: true,
	}, nil
}
