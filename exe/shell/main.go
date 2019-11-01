package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	cbornode "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	swarm "github.com/libp2p/go-libp2p-swarm"
	"github.com/libp2p/go-libp2p/p2p/discovery"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
	t "github.com/textileio/go-textile-threads"
	"github.com/textileio/go-textile-threads/cbor"
	"github.com/textileio/go-textile-threads/exe/util"
	tutil "github.com/textileio/go-textile-threads/util"
)

var (
	ctx      context.Context
	ds       datastore.Batching
	dht      *kaddht.IpfsDHT
	api      tserv.Threadservice
	threadID thread.ID

	grey   = color.New(color.FgHiBlack).SprintFunc()
	green  = color.New(color.FgHiGreen).SprintFunc()
	cyan   = color.New(color.FgHiCyan).SprintFunc()
	yellow = color.New(color.FgHiYellow).SprintFunc()
	red    = color.New(color.FgHiRed).SprintFunc()

	bootstrapPeers = []string{
		"/ip4/104.210.43.77/tcp/4001/ipfs/12D3KooWSdGmRz5JQidqrtmiPGVHkStXpbSAMnbCcW8abq6zuiDP", // us-west
		"/ip4/20.39.232.27/tcp/4001/ipfs/12D3KooWLnUv9MWuRM6uHirRPBM4NwRj54n4gNNnBtiFiwPiv3Up",  // eu-west
		"/ip4/34.87.103.105/tcp/4001/ipfs/12D3KooWA5z2C3z1PNKi36Bw1MxZhBD8nv7UbB7YQP6WcSWYNwRQ", // as-southeast
	}
)

const (
	msgTimeout      = time.Second * 10
	findPeerTimeout = time.Second * 30
	timeLayout      = "03:04:05 PM"
)

func init() {
	cbornode.RegisterCborType(msg{})
}

var log = logging.Logger("shell")

type msg struct {
	Txt string
}

func main() {
	var cancel context.CancelFunc
	var h host.Host
	ctx, cancel, ds, h, dht, api = util.Build(bootstrapPeers)

	defer cancel()
	defer h.Close()
	defer dht.Close()
	defer api.Close()

	// Build a MDNS service
	mdns, err := discovery.NewMdnsService(ctx, h, time.Second, "")
	if err != nil {
		panic(err)
	}
	defer mdns.Close()

	// Start the prompt
	fmt.Println(grey("Welcome to Threads!"))
	fmt.Println(grey("Your peer ID is ") + yellow(h.ID().String()))
	cursor := green(">  ")

	sub := api.Subscribe()
	go func() {
		for {
			select {
			case rec, ok := <-sub.Channel():
				if !ok {
					return
				}
				if rec.ThreadID().String() != threadID.String() {
					continue
				}

				event, err := cbor.EventFromRecord(ctx, api, rec.Value())
				if err != nil {
					logError(err)
					continue
				}
				key, err := api.Store().ReadKey(rec.ThreadID(), rec.LogID())
				if err != nil {
					logError(err)
					continue
				}
				if key == nil {
					continue
				}
				node, err := event.GetBody(ctx, api, key)
				if err != nil {
					continue // Not for us
				}
				m := new(msg)
				err = cbornode.DecodeInto(node.RawData(), m)
				if err != nil {
					continue // Not one of our messages
				}
				header, err := event.GetHeader(ctx, api, key)
				if err != nil {
					logError(err)
					continue
				}
				msgTime, err := header.Time()
				if err != nil {
					logError(err)
					continue
				}

				fmt.Println(grey(msgTime.Format(timeLayout)+" ") +
					cyan(shortID(rec.LogID())+"  ") +
					grey(m.Txt))

				fmt.Print(cursor)
			}
		}
	}()

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(cursor)
		line, err := reader.ReadString('\n')
		if err != nil {
			panic(err)
		}
		clean(1)

		out, err := handleLine(line)
		if err != nil {
			logError(err)
		}
		if out != "" {
			fmt.Println(grey(out))
		}
	}
}

func clean(lineCnt int) {
	buf := bufio.NewWriter(os.Stdout)
	_, _ = buf.Write([]byte("\033[J"))
	for i := 0; i < lineCnt; i++ {
		_, _ = io.WriteString(buf, "\033[2K\r\033[A")
	}
	_, _ = io.WriteString(buf, "\033[2K\r")
	_ = buf.Flush()
	return
}

func handleLine(line string) (out string, err error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	if strings.HasPrefix(line, ":") {
		args := strings.Split(line[1:], " ")
		switch args[0] {
		case "address":
			return addressCmd()
		case "thread":
			return threadCmd(args[1:])
		case "threads":
			return threadsCmd()
		case "thread-address":
			return threadAddressCmd()
		case "add-follower":
			return addFollowerCmd(args[1:])
		default:
			err = fmt.Errorf("unknown command: %s", args[0])
			return
		}
	}

	err = sendMessage(line)
	return
}

func addressCmd() (out string, err error) {
	pro := ma.ProtocolWithCode(ma.P_P2P).Name
	addr, err := ma.NewMultiaddr("/" + pro + "/" + api.Host().ID().String())
	if err != nil {
		return
	}
	addrs := api.Host().Addrs()
	for i, a := range addrs {
		a = a.Encapsulate(addr)
		out += a.String()
		if i != len(addrs)-1 {
			out += "\n"
		}
	}
	return
}

func threadCmd(args []string) (out string, err error) {
	if len(args) == 0 {
		err = fmt.Errorf("enter a thread name to use")
		return
	}
	name := args[0]

	var addr ma.Multiaddr
	if len(args) > 1 {
		addr, err = ma.NewMultiaddr(args[1])
		if err != nil {
			return
		}
		if !canDial(addr) {
			return "", fmt.Errorf("address is not dialable")
		}
	}

	x, err := ds.Get(datastore.NewKey("/names/" + name))
	if err == datastore.ErrNotFound {
		if addr != nil {
			info, err := api.AddThread(ctx, addr)
			if err != nil {
				return "", err
			}
			threadID = info.ID
		} else {
			threadID = thread.NewIDV1(thread.Raw, 32)
		}
		err = ds.Put(datastore.NewKey("/names/"+name), threadID.Bytes())
		if err != nil {
			return
		}
	} else if err != nil {
		return
	} else {
		threadID, err = thread.Cast(x)
		if err != nil {
			return
		}
	}

	out = fmt.Sprintf("Using thread %s", threadID.String())
	return
}

func threadsCmd() (out string, err error) {
	q, err := ds.Query(query.Query{Prefix: "/names"})
	if err != nil {
		return
	}
	all, err := q.Rest()
	if err != nil {
		return
	}

	var id thread.ID
	for i, e := range all {
		id, err = thread.Cast(e.Value)
		if err != nil {
			return
		}
		name := e.Key[strings.LastIndex(e.Key, "/")+1:]
		out += name + ": " + id.String()
		if i != len(all)-1 {
			out += "\n"
		}
	}
	return
}

func threadAddressCmd() (out string, err error) {
	if !threadID.Defined() {
		err = fmt.Errorf("choose a thread to use with `:thread`")
		return
	}

	lg, err := tutil.GetOwnLog(api, threadID)
	if err != nil {
		return
	}
	ta, err := ma.NewMultiaddr("/" + t.Thread + "/" + threadID.String())
	if err != nil {
		return
	}

	var addrs []ma.Multiaddr
	for _, la := range lg.Addrs {
		p2p, err := la.ValueForProtocol(ma.P_P2P)
		if err != nil {
			return "", err
		}
		pid, err := peer.IDB58Decode(p2p)
		if err != nil {
			return "", err
		}

		var paddrs []ma.Multiaddr
		if pid.String() == api.Host().ID().String() {
			paddrs = api.Host().Addrs()
		} else {
			paddrs = api.Host().Peerstore().Addrs(pid)
		}
		for _, pa := range paddrs {
			addrs = append(addrs, pa.Encapsulate(la).Encapsulate(ta))
		}
	}

	for i, a := range addrs {
		out += a.String()
		if i != len(addrs)-1 {
			out += "\n"
		}
	}

	return
}

func addFollowerCmd(args []string) (out string, err error) {
	if !threadID.Defined() {
		err = fmt.Errorf("choose a thread to use with `:thread`")
		return
	}

	if len(args) == 0 {
		err = fmt.Errorf("enter a peer address")
		return
	}

	addr, err := ma.NewMultiaddr(args[0])
	if err != nil {
		return
	}
	p2p, err := addr.ValueForProtocol(ma.P_P2P)
	if err != nil {
		return
	}
	pid, err := peer.IDB58Decode(p2p)
	if err != nil {
		return
	}
	dialable, err := getDialable(addr)
	if err != nil {
		return
	}

	if dialable != nil {
		api.Host().Peerstore().AddAddr(pid, dialable, peerstore.PermanentAddrTTL)
	}
	if len(api.Host().Peerstore().Addrs(pid)) == 0 {
		pctx, cancel := context.WithTimeout(ctx, findPeerTimeout)
		defer cancel()
		pinfo, err := dht.FindPeer(pctx, pid)
		if err != nil {
			return "", err
		}
		api.Host().Peerstore().AddAddrs(pid, pinfo.Addrs, peerstore.PermanentAddrTTL)
	}

	err = api.AddFollower(ctx, threadID, pid)
	return
}

func sendMessage(txt string) error {
	if !threadID.Defined() {
		return fmt.Errorf("choose a thread to use with `:thread`")
	}

	body, err := cbornode.WrapObject(&msg{Txt: txt}, mh.SHA2_256, -1)
	if err != nil {
		return err
	}
	go func() {
		mctx, cancel := context.WithTimeout(ctx, msgTimeout)
		defer cancel()
		_, err = api.AddRecord(mctx, body, tserv.AddOpt.ThreadID(threadID))
		if err != nil {
			log.Errorf("error writing message: %s", err)
		}
	}()
	return nil
}

func shortID(id peer.ID) string {
	l := id.String()
	return l[len(l)-7:]
}

func getDialable(addr ma.Multiaddr) (ma.Multiaddr, error) {
	parts := strings.Split(addr.String(), "/"+ma.ProtocolWithCode(ma.P_P2P).Name)
	return ma.NewMultiaddr(parts[0])
}

func canDial(addr ma.Multiaddr) bool {
	parts := strings.Split(addr.String(), "/"+ma.ProtocolWithCode(ma.P_P2P).Name)
	addr, _ = ma.NewMultiaddr(parts[0])
	tr := api.Host().Network().(*swarm.Swarm).TransportForDialing(addr)
	return tr != nil && tr.CanDial(addr)
}

func logError(err error) {
	fmt.Println(red("Error: " + err.Error()))
}
