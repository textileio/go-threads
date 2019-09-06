package cbor

import (
	blocks "github.com/ipfs/go-block-format"
	node "github.com/ipfs/go-ipld-cbor"
	format "github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-textile-core/crypto"
)

// EncodeBlock returns a node by encrypting the block's raw bytes with key.
func EncodeBlock(block blocks.Block, key []byte) (format.Node, error) {
	ciphertext, err := crypto.EncryptAES(block.RawData(), key)
	if err != nil {
		return nil, err
	}
	return node.WrapObject(ciphertext, mh.SHA2_256, -1)
}

// DecodeBlock returns a node by decrypting the block's raw bytes with key.
func DecodeBlock(block blocks.Block, key []byte) (format.Node, error) {
	plaintext, err := crypto.DecryptAES(block.RawData(), key)
	if err != nil {
		return nil, err
	}
	return node.Decode(plaintext, mh.SHA2_256, -1)
}
