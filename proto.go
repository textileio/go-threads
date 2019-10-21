package threads

import (
	"fmt"

	"github.com/libp2p/go-libp2p-core/protocol"
	ma "github.com/multiformats/go-multiaddr"
	mb "github.com/multiformats/go-multibase"
	"github.com/textileio/go-textile-core/thread"
)

const (
	// Thread is the protocol slug.
	Thread = "thread"
	// ThreadCode is the protocol code.
	ThreadCode = 406
	// ThreadsVersion is the current protocol version.
	ThreadVersion = "0.0.1"
	// ThreadProtocol is the threads protocol tag.
	ThreadProtocol protocol.ID = "/" + Thread + "/" + ThreadVersion
)

var addrProtocol = ma.Protocol{
	Name:       Thread,
	Code:       ThreadCode,
	VCode:      ma.CodeToVarint(ThreadCode),
	Size:       ma.LengthPrefixedVarSize,
	Transcoder: TranscoderThread,
}

var TranscoderThread = ma.NewTranscoderFromFunctions(threadStB, threadBtS, threadVal)

func threadStB(s string) ([]byte, error) {
	_, data, err := mb.Decode(s)
	if err != nil {
		return nil, fmt.Errorf("failed to parse thread addr: %s %s", s, err)
	}
	return data, nil
}

func threadVal(b []byte) error {
	_, err := thread.Cast(b)
	return err
}

func threadBtS(b []byte) (string, error) {
	m, err := thread.Cast(b)
	if err != nil {
		return "", err
	}
	return m.String(), nil
}

func init() {
	if err := ma.AddProtocol(addrProtocol); err != nil {
		panic(err)
	}
}
