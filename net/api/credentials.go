package api

import (
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/textileio/go-threads/core/thread"
	pb "github.com/textileio/go-threads/net/api/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type credentials struct {
	threadID  thread.ID
	pubKey    crypto.PubKey
	signature thread.Signature
}

func newCredentials(c *pb.Credentials) (creds credentials, err error) {
	creds.threadID, err = thread.Cast(c.ThreadID)
	if err != nil {
		err = status.Error(codes.InvalidArgument, err.Error())
		return
	}
	if c.PubKey == nil {
		return
	}
	key, err := crypto.UnmarshalPublicKey(c.PubKey)
	if err != nil {
		err = status.Error(codes.InvalidArgument, err.Error())
		return
	}
	return credentials{
		pubKey:    key,
		signature: c.Signature,
	}, nil
}

func (c credentials) ThreadID() thread.ID {
	return c.threadID
}

func (c credentials) Sign() (crypto.PubKey, thread.Signature, error) {
	return c.pubKey, c.signature, nil
}
