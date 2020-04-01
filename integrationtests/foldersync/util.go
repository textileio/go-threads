package foldersync

import (
	"strings"

	"github.com/mr-tron/base58"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/core/thread"
)

func parseInviteLink(inviteLink string) (thread.ID, ma.Multiaddr, thread.Key) {
	addrRest := strings.Split(inviteLink, "?")
	addr, err := ma.NewMultiaddr(addrRest[0])
	if err != nil {
		panic("invalid invite link")
	}
	keyThreadID := strings.Split(addrRest[1], "&")
	keyBytes, err := base58.Decode(keyThreadID[0])
	if err != nil {
		panic("invalid key")
	}
	key, err := thread.KeyFromBytes(keyBytes)
	if err != nil {
		panic("invalid key")
	}
	threadID, err := thread.Decode(keyThreadID[1])
	if err != nil {
		panic("invalid thread id")
	}
	return threadID, addr, key
}
