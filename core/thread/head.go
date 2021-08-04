package thread

import "github.com/ipfs/go-cid"

// Head represents the log head (including the number of records in the log and the id of the head)
type Head struct {
	// ID of the head
	ID      cid.Cid
	// Counter is the number of logs in the head
	Counter int64
}

const CounterUndef int64 = 0

var HeadUndef = Head{
	ID:      cid.Undef,
	Counter: CounterUndef,
}
