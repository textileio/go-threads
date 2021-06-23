package thread

import "github.com/ipfs/go-cid"

type Head struct {
	ID      cid.Cid
	Counter int64
}

const CounterUndef int64 = 0

var HeadUndef = Head{
	ID:      cid.Undef,
	Counter: CounterUndef,
}
