package thread

import (
	"fmt"

	"github.com/libp2p/go-libp2p-core/protocol"
	ma "github.com/multiformats/go-multiaddr"
	mb "github.com/multiformats/go-multibase"
)

const (
	// Name is the protocol slug.
	Name = "thread"
	// Code is the protocol code.
	Code = 406
	// Version is the current protocol version.
	Version = "0.0.1"
	// Protocol is the threads protocol tag.
	Protocol protocol.ID = "/" + Name + "/" + Version
)

var addrProtocol = ma.Protocol{
	Name:       Name,
	Code:       Code,
	VCode:      ma.CodeToVarint(Code),
	Size:       ma.LengthPrefixedVarSize,
	Transcoder: ma.NewTranscoderFromFunctions(threadStB, threadBtS, threadVal),
}

func threadStB(s string) ([]byte, error) {
	_, data, err := mb.Decode(s)
	if err != nil {
		return nil, fmt.Errorf("failed to parse thread addr: %s %s", s, err)
	}
	return data, nil
}

func threadVal(b []byte) error {
	_, err := Cast(b)
	return err
}

func threadBtS(b []byte) (string, error) {
	m, err := Cast(b)
	if err != nil {
		return "", err
	}
	if err := m.Validate(); err != nil {
		return "", err
	}
	return m.String(), nil
}

func init() {
	if err := ma.AddProtocol(addrProtocol); err != nil {
		panic(err)
	}
}
