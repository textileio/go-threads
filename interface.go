package threads

import (
	"io"

	"github.com/ipfs/go-cid"
	pb "github.com/textileio/go-textile-threads/pb"
	"github.com/textileio/go-textile-wallet/account"
)

type ThreadIntent string

type Thread interface {
	ID() string
	Key() []byte
	Schema() *pb.SchemaNode
	Intent() ThreadIntent
	Public() bool
	Roles() *pb.Roles

	Heads() []cid.Cid

	GetName() string
	SetName() error

	CreateInvite() (cid.Cid, error)
	Invite(account.Account) error

	Join() error
	Leave() error

	Write(io.Reader /*, WriteOpts */) (cid.Cid, error)
	Listen()

	Fork() (Thread, error)
}

type Threadstore interface {
	io.Closer

	Put(cid.Cid, pb.Index) (Thread, error)
	Get(cid.Cid) Thread

	Threads() []Thread
	ThreadInfo(cid.Cid)
}
