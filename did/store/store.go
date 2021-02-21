package store

import (
	"encoding/json"

	ds "github.com/ipfs/go-datastore"
	"github.com/textileio/go-threads/core/did"
)

type Store struct {
	s ds.Datastore
}

func NewStore(s ds.Datastore) *Store {
	return &Store{s: s}
}

func (s *Store) Put(did did.DID, document did.Document) error {
	v, err := json.Marshal(document)
	if err != nil {
		return err
	}
	return s.s.Put(ds.NewKey(string(did)), v)
}

func (s *Store) Get(did did.DID) (doc did.Document, err error) {
	v, err := s.s.Get(ds.NewKey(string(did)))
	if err != nil {
		return doc, err
	}
	if err := json.Unmarshal(v, &doc); err != nil {
		return doc, err
	}
	return doc, err
}

func (s *Store) Delete(did did.DID) error {
	return s.s.Delete(ds.NewKey(string(did)))
}
