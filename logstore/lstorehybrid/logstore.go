package lstorehybrid

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	core "github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
	sym "github.com/textileio/go-threads/crypto/symmetric"
)

var _ core.Logstore = (*lstore)(nil)

type lstore struct {
	inMem, persist core.Logstore
}

func NewLogstore(persist, inMem core.Logstore) (*lstore, error) {
	// on a start it's required to synchronize both storages, so we
	// initialize in-memory storage with data from the persistent one
	dKeys, err := persist.DumpKeys()
	if err != nil {
		return nil, fmt.Errorf("dumping keys from persistent storage: %w", err)
	}
	dAddrs, err := persist.DumpAddrs()
	if err != nil {
		return nil, fmt.Errorf("dumping addresses from persistent storage: %w", err)
	}
	dHeads, err := persist.DumpHeads()
	if err != nil {
		return nil, fmt.Errorf("dumping heads from persistent storage: %w", err)
	}
	dMeta, err := persist.DumpMeta()
	if err != nil {
		return nil, fmt.Errorf("dumping metadata from persistent storage: %w", err)
	}

	// initialize in-memory storage
	if err := inMem.RestoreKeys(dKeys); err != nil {
		return nil, fmt.Errorf("initializing in-memory storage with keys: %w", err)
	}
	if err := inMem.RestoreAddrs(dAddrs); err != nil {
		return nil, fmt.Errorf("initializing in-memory storage with addresses: %w", err)
	}
	if err := inMem.RestoreHeads(dHeads); err != nil {
		return nil, fmt.Errorf("initializing in-memory storage with heads: %w", err)
	}
	if err := inMem.RestoreMeta(dMeta); err != nil {
		return nil, fmt.Errorf("initializing in-memory storage with metadata: %w", err)
	}

	return &lstore{inMem: inMem, persist: persist}, nil
}

func (l *lstore) Close() error {
	if err := l.persist.Close(); err != nil {
		return err
	}
	return l.inMem.Close()
}

func (l *lstore) GetInt64(tid thread.ID, key string) (*int64, error) {
	return l.inMem.GetInt64(tid, key)
}

func (l *lstore) PutInt64(tid thread.ID, key string, val int64) error {
	if err := l.persist.PutInt64(tid, key, val); err != nil {
		return err
	}
	return l.inMem.PutInt64(tid, key, val)
}

func (l *lstore) GetString(tid thread.ID, key string) (*string, error) {
	return l.inMem.GetString(tid, key)
}

func (l *lstore) PutString(tid thread.ID, key string, val string) error {
	if err := l.persist.PutString(tid, key, val); err != nil {
		return err
	}
	return l.inMem.PutString(tid, key, val)
}

func (l *lstore) GetBool(tid thread.ID, key string) (*bool, error) {
	return l.inMem.GetBool(tid, key)
}

func (l *lstore) PutBool(tid thread.ID, key string, val bool) error {
	if err := l.persist.PutBool(tid, key, val); err != nil {
		return err
	}
	return l.inMem.PutBool(tid, key, val)
}

func (l *lstore) GetBytes(tid thread.ID, key string) (*[]byte, error) {
	return l.inMem.GetBytes(tid, key)
}

func (l *lstore) PutBytes(tid thread.ID, key string, val []byte) error {
	if err := l.persist.PutBytes(tid, key, val); err != nil {
		return err
	}
	return l.inMem.PutBytes(tid, key, val)
}

func (l *lstore) ClearMetadata(tid thread.ID) error {
	if err := l.persist.ClearMetadata(tid); err != nil {
		return err
	}
	return l.inMem.ClearMetadata(tid)
}

func (l *lstore) PubKey(tid thread.ID, lid peer.ID) (crypto.PubKey, error) {
	return l.inMem.PubKey(tid, lid)
}

func (l *lstore) AddPubKey(tid thread.ID, lid peer.ID, key crypto.PubKey) error {
	if err := l.persist.AddPubKey(tid, lid, key); err != nil {
		return err
	}
	return l.inMem.AddPubKey(tid, lid, key)
}

func (l *lstore) PrivKey(tid thread.ID, lid peer.ID) (crypto.PrivKey, error) {
	return l.inMem.PrivKey(tid, lid)
}

func (l *lstore) AddPrivKey(tid thread.ID, lid peer.ID, key crypto.PrivKey) error {
	if err := l.persist.AddPrivKey(tid, lid, key); err != nil {
		return err
	}
	return l.inMem.AddPrivKey(tid, lid, key)
}

func (l *lstore) ReadKey(tid thread.ID) (*sym.Key, error) {
	return l.inMem.ReadKey(tid)
}

func (l *lstore) AddReadKey(tid thread.ID, key *sym.Key) error {
	if err := l.persist.AddReadKey(tid, key); err != nil {
		return err
	}
	return l.inMem.AddReadKey(tid, key)
}

func (l *lstore) ServiceKey(tid thread.ID) (*sym.Key, error) {
	return l.inMem.ServiceKey(tid)
}

func (l *lstore) AddServiceKey(tid thread.ID, key *sym.Key) error {
	if err := l.persist.AddServiceKey(tid, key); err != nil {
		return err
	}
	return l.inMem.AddServiceKey(tid, key)
}

func (l *lstore) ClearKeys(tid thread.ID) error {
	if err := l.persist.ClearKeys(tid); err != nil {
		return err
	}
	return l.inMem.ClearKeys(tid)
}

func (l *lstore) ClearLogKeys(tid thread.ID, lid peer.ID) error {
	if err := l.persist.ClearLogKeys(tid, lid); err != nil {
		return err
	}
	return l.inMem.ClearLogKeys(tid, lid)
}

func (l *lstore) LogsWithKeys(tid thread.ID) (peer.IDSlice, error) {
	return l.inMem.LogsWithKeys(tid)
}

func (l *lstore) ThreadsFromKeys() (thread.IDSlice, error) {
	return l.inMem.ThreadsFromKeys()
}

func (l *lstore) AddAddr(tid thread.ID, lid peer.ID, addr ma.Multiaddr, dur time.Duration) error {
	if err := l.persist.AddAddr(tid, lid, addr, dur); err != nil {
		return err
	}
	return l.inMem.AddAddr(tid, lid, addr, dur)
}

func (l *lstore) AddAddrs(tid thread.ID, lid peer.ID, addrs []ma.Multiaddr, dur time.Duration) error {
	if err := l.persist.AddAddrs(tid, lid, addrs, dur); err != nil {
		return err
	}
	return l.inMem.AddAddrs(tid, lid, addrs, dur)
}

func (l *lstore) SetAddr(tid thread.ID, lid peer.ID, addr ma.Multiaddr, dur time.Duration) error {
	if err := l.persist.SetAddr(tid, lid, addr, dur); err != nil {
		return err
	}
	return l.inMem.SetAddr(tid, lid, addr, dur)
}

func (l *lstore) SetAddrs(tid thread.ID, lid peer.ID, addrs []ma.Multiaddr, dur time.Duration) error {
	if err := l.persist.SetAddrs(tid, lid, addrs, dur); err != nil {
		return err
	}
	return l.inMem.SetAddrs(tid, lid, addrs, dur)
}

func (l *lstore) UpdateAddrs(tid thread.ID, lid peer.ID, oldTTL time.Duration, newTTL time.Duration) error {
	if err := l.persist.UpdateAddrs(tid, lid, oldTTL, newTTL); err != nil {
		return err
	}
	return l.inMem.UpdateAddrs(tid, lid, oldTTL, newTTL)
}

func (l *lstore) Addrs(tid thread.ID, lid peer.ID) ([]ma.Multiaddr, error) {
	return l.inMem.Addrs(tid, lid)
}

func (l *lstore) AddrStream(ctx context.Context, tid thread.ID, lid peer.ID) (<-chan ma.Multiaddr, error) {
	return l.inMem.AddrStream(ctx, tid, lid)
}

func (l *lstore) ClearAddrs(tid thread.ID, lid peer.ID) error {
	if err := l.persist.ClearAddrs(tid, lid); err != nil {
		return err
	}
	return l.inMem.ClearAddrs(tid, lid)
}

func (l *lstore) LogsWithAddrs(tid thread.ID) (peer.IDSlice, error) {
	return l.inMem.LogsWithAddrs(tid)
}

func (l *lstore) ThreadsFromAddrs() (thread.IDSlice, error) {
	return l.inMem.ThreadsFromAddrs()
}

func (l *lstore) AddHead(tid thread.ID, lid peer.ID, cid cid.Cid) error {
	if err := l.persist.AddHead(tid, lid, cid); err != nil {
		return err
	}
	return l.inMem.AddHead(tid, lid, cid)
}

func (l *lstore) AddHeads(tid thread.ID, lid peer.ID, cids []cid.Cid) error {
	if err := l.persist.AddHeads(tid, lid, cids); err != nil {
		return err
	}
	return l.inMem.AddHeads(tid, lid, cids)
}

func (l *lstore) SetHead(tid thread.ID, lid peer.ID, cid cid.Cid) error {
	if err := l.persist.SetHead(tid, lid, cid); err != nil {
		return err
	}
	return l.inMem.SetHead(tid, lid, cid)
}

func (l *lstore) SetHeads(tid thread.ID, lid peer.ID, cids []cid.Cid) error {
	if err := l.persist.SetHeads(tid, lid, cids); err != nil {
		return err
	}
	return l.inMem.SetHeads(tid, lid, cids)
}

func (l *lstore) Heads(tid thread.ID, lid peer.ID) ([]cid.Cid, error) {
	return l.inMem.Heads(tid, lid)
}

func (l *lstore) ClearHeads(tid thread.ID, lid peer.ID) error {
	if err := l.persist.ClearHeads(tid, lid); err != nil {
		return err
	}
	return l.inMem.ClearHeads(tid, lid)
}

func (l *lstore) Threads() (thread.IDSlice, error) {
	return l.inMem.Threads()
}

func (l *lstore) AddThread(info thread.Info) error {
	if err := l.persist.AddThread(info); err != nil {
		return err
	}
	return l.inMem.AddThread(info)
}

func (l *lstore) GetThread(tid thread.ID) (thread.Info, error) {
	return l.inMem.GetThread(tid)
}

func (l *lstore) DeleteThread(tid thread.ID) error {
	if err := l.persist.DeleteThread(tid); err != nil {
		return err
	}
	return l.inMem.DeleteThread(tid)
}

func (l *lstore) AddLog(tid thread.ID, info thread.LogInfo) error {
	if err := l.persist.AddLog(tid, info); err != nil {
		return err
	}
	return l.inMem.AddLog(tid, info)
}

func (l *lstore) GetLog(tid thread.ID, lid peer.ID) (thread.LogInfo, error) {
	return l.inMem.GetLog(tid, lid)
}

func (l *lstore) GetManagedLogs(tid thread.ID) ([]thread.LogInfo, error) {
	return l.inMem.GetManagedLogs(tid)
}

func (l *lstore) DeleteLog(tid thread.ID, lid peer.ID) error {
	if err := l.persist.DeleteLog(tid, lid); err != nil {
		return err
	}
	return l.inMem.DeleteLog(tid, lid)
}

func (l *lstore) DumpMeta() (core.DumpMetadata, error) {
	return l.inMem.DumpMeta()
}

func (l *lstore) RestoreMeta(dump core.DumpMetadata) error {
	if err := l.persist.RestoreMeta(dump); err != nil {
		return err
	}
	return l.inMem.RestoreMeta(dump)
}

func (l *lstore) DumpKeys() (core.DumpKeyBook, error) {
	return l.inMem.DumpKeys()
}

func (l *lstore) RestoreKeys(dump core.DumpKeyBook) error {
	if err := l.persist.RestoreKeys(dump); err != nil {
		return err
	}
	return l.inMem.RestoreKeys(dump)
}

func (l *lstore) DumpAddrs() (core.DumpAddrBook, error) {
	return l.inMem.DumpAddrs()
}

func (l *lstore) RestoreAddrs(dump core.DumpAddrBook) error {
	if err := l.persist.RestoreAddrs(dump); err != nil {
		return err
	}
	return l.inMem.RestoreAddrs(dump)
}

func (l *lstore) DumpHeads() (core.DumpHeadBook, error) {
	return l.inMem.DumpHeads()
}

func (l *lstore) RestoreHeads(dump core.DumpHeadBook) error {
	if err := l.persist.RestoreHeads(dump); err != nil {
		return err
	}
	return l.inMem.RestoreHeads(dump)
}
