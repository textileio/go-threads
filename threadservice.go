package threads

import (
	format "github.com/ipfs/go-ipld-format"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
	tstore "github.com/textileio/go-textile-core/threadstore"
)

type threadservice struct {
	ic.PrivKey
	host.Host
	format.DAGService
	tstore.Threadstore
}

func NewThreadservice(sk ic.PrivKey, h host.Host, ds format.DAGService, ts tstore.Threadstore) tserv.Threadservice {
	return &threadservice{
		PrivKey:     sk,
		Host:        h,
		DAGService:  ds,
		Threadstore: ts,
	}
}

func (ts *threadservice) Put(event thread.Event, t ...thread.ID) peer.IDSlice {
	panic("implement me")
}

func (ts *threadservice) Pull(offset string, size int, t thread.ID) <-chan []thread.Event {
	panic("implement me")
}

func (ts *threadservice) Invite(actor peer.ID, t thread.ID) error {
	panic("implement me")
}

func (ts *threadservice) Leave(thread.ID) error {
	panic("implement me")
}

func (ts *threadservice) Delete(thread.ID) error {
	panic("implement me")
}
