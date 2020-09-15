package cbor

import (
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-ipld-cbor"
	"github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-threads/crypto"
)

// EncodeBlock returns a node by encrypting the block's raw bytes with key.
func EncodeBlock(block blocks.Block, key crypto.EncryptionKey) (format.Node, error) {
	coded, err := key.Encrypt(block.RawData())
	if err != nil {
		return nil, err
	}
	return cbornode.WrapObject(coded, mh.SHA2_256, -1)
}

// DecodeBlock returns a node by decrypting the block's raw bytes with key.
func DecodeBlock(block blocks.Block, key crypto.DecryptionKey) (format.Node, error) {
	var raw []byte
	err := cbornode.DecodeInto(block.RawData(), &raw)
	if err != nil {
		return nil, err
	}
	decoded, err := key.Decrypt(raw)
	if err != nil {
		return nil, err
	}
	return cbornode.Decode(decoded, mh.SHA2_256, -1)
}
