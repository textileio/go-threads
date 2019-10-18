package main

import (
	"context"
	"flag"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/fatih/color"
	ipfslite "github.com/hsanjuan/ipfs-lite"
	"github.com/ipfs/go-datastore"
	cbornode "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/p2p/discovery"
	"github.com/mitchellh/go-homedir"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-textile-core/crypto/asymmetric"
	"github.com/textileio/go-textile-core/crypto/symmetric"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
	t "github.com/textileio/go-textile-threads"
	"github.com/textileio/go-textile-threads/cbor"
	"github.com/textileio/go-textile-threads/tstoreds"
	"github.com/textileio/go-textile-threads/util"
	"gopkg.in/natefinch/lumberjack.v2"
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
)

func init() {
	cbornode.RegisterCborType(msg{})
}

type msg struct {
	Txt string
}

func main() {
	repo := flag.String("repo", "threads", "repo location")
	port := flag.Int("port", 4006, "host port")
	flag.Parse()

	repop, err := homedir.Expand(*repo)
	if err != nil {
		panic(err)
	}

	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	// Build an IPFS-Lite peer
	ds, err = ipfslite.BadgerDatastore(repop)
	if err != nil {
		panic(err)
	}
	priv, _, err := ic.GenerateKeyPair(ic.Ed25519, 0)
	if err != nil {
		panic(err)
	}
	listen, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *port))
	if err != nil {
		panic(err)
	}

	connman := connmgr.NewConnManager(100, 400, time.Minute)
	var h host.Host
	h, dht, err = ipfslite.SetupLibp2p(
		ctx,
		priv,
		nil,
		[]ma.Multiaddr{listen},
		libp2p.ConnectionManager(connman),
	)
	if err != nil {
		panic(err)
	}
	defer h.Close()
	defer dht.Close()

	lite, err := ipfslite.New(ctx, ds, h, dht, nil)
	if err != nil {
		panic(err)
	}

	// Build a MDNS service
	mdns, err := discovery.NewMdnsService(ctx, h, time.Second, "")
	if err != nil {
		panic(err)
	}
	defer mdns.Close()

	// Build a threadstore
	tstore, err := tstoreds.NewThreadstore(ctx, ds, tstoreds.DefaultOpts())
	if err != nil {
		panic(err)
	}

	// Build a threadservice
	writer := &lumberjack.Logger{
		Filename:   path.Join(repop, "log"),
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     30, // days
	}
	api, err = t.NewThreads(ctx, h, lite.BlockStore(), lite, tstore, writer, true)
	if err != nil {
		panic(err)
	}
	defer api.Close()

	// Bootstrap to textile peers
	err = logging.SetLogLevel("ipfslite", "debug")
	if err != nil {
		panic(err)
	}
	boots, err := parseBootstrapPeers(bootstrapPeers)
	if err != nil {
		panic(err)
	}
	lite.Bootstrap(boots)

	// Start the prompt
	fmt.Println(grey("Welcome to Threads!"))
	fmt.Println(grey("Your peer ID is ") + yellow(h.ID().String()))

	nick := h.ID().String()[len(h.ID().String())-7:]

	rl, err := readline.New(green("you  "))
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	sub := api.Listen()
	last := true
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

				lg, err := util.GetOwnLog(api, threadID)
				if err != nil {
					logError(err)
					continue
				}

				if lg.ID.String() == rec.LogID().String() {
					continue // This is our record
				}

				event, err := cbor.EventFromRecord(ctx, api.DAGService(), rec.Value())
				if err != nil {
					logError(err)
					continue
				}
				rk, err := api.ReadKey(rec.ThreadID(), rec.LogID())
				if err != nil {
					logError(err)
					continue
				}
				if rk == nil {
					continue
				}
				key, err := symmetric.NewKey(rk)
				if err != nil {
					logError(err)
					continue
				}
				node, err := event.GetBody(ctx, api.DAGService(), key)
				if err != nil {
					continue // Not for us
				}
				m := new(msg)
				err = cbornode.DecodeInto(node.RawData(), m)
				if err != nil {
					continue // Not one of our messages
				}

				if last {
					fmt.Println()
				}
				fmt.Println(cyan(nick) + "  " + grey(m.Txt))
				last = false
			}
		}
	}()

	for {
		line, err := rl.Readline()
		if err != nil {
			break
		}

		out, err := handleLine(line)
		if err != nil {
			logError(err)
		}
		if out != "" {
			fmt.Println(grey(out))
		}
		last = true
	}
}

func handleLine(line string) (out string, err error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	if strings.HasPrefix(line, ":") {
		args := strings.Split(line[1:], " ")
		switch args[0] {
		case "use":
			return useCmd(args[1:])
		case "add":
			return addCmd(args[1:])
		default:
			err = fmt.Errorf("unknown command: %s\n", args[0])
			return
		}
	}

	err = sendMessage(line)
	return
}

func useCmd(args []string) (out string, err error) {
	if len(args) == 0 {
		err = fmt.Errorf("enter a thread name to use")
		return
	}
	name := args[0]

	x, err := ds.Get(datastore.NewKey("/names/" + name))
	if err == datastore.ErrNotFound {
		threadID = thread.NewIDV1(thread.Raw, 32)
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

func addCmd(args []string) (out string, err error) {
	if len(args) == 0 {
		err = fmt.Errorf("enter a thread name to use")
		return
	}
	p := args[0]

	pid, err := peer.IDB58Decode(p)
	if err != nil {
		return
	}

	pctx, cancel := context.WithTimeout(ctx, findPeerTimeout)
	defer cancel()
	pinfo, err := dht.FindPeer(pctx, pid)
	if err != nil {
		return
	}
	api.Host().Peerstore().AddAddrs(pid, pinfo.Addrs, peerstore.PermanentAddrTTL)

	pk, err := pid.ExtractPublicKey()
	if err != nil {
		return
	}

	lgs, err := api.GetLogs(threadID)
	if err != nil {
		return
	}
	if len(lgs) == 0 {
		err = fmt.Errorf("thread in use is empty")
		return
	}
	logs, err := cbor.NewLogs(lgs, true)
	if err != nil {
		return
	}

	ek, err := asymmetric.NewEncryptionKey(pk)
	if err != nil {
		return
	}

	a, err := ma.NewMultiaddr("/p2p/" + pid.String())
	if err != nil {
		return
	}

	_, err = api.Add(
		ctx,
		logs,
		tserv.AddOpt.ThreadID(threadID),
		tserv.AddOpt.Key(ek),
		tserv.AddOpt.Addrs([]ma.Multiaddr{a}))
	return
}

func sendMessage(txt string) error {
	if !threadID.Defined() {
		return fmt.Errorf("choose a thread to use with `:use`")
	}

	mctx, cancel := context.WithTimeout(ctx, msgTimeout)
	defer cancel()

	body, err := cbornode.WrapObject(&msg{Txt: txt}, mh.SHA2_256, -1)
	if err != nil {
		return err
	}
	_, err = api.Add(mctx, body, tserv.AddOpt.ThreadID(threadID))
	return err
}

func parseBootstrapPeers(addrs []string) ([]peer.AddrInfo, error) {
	maddrs := make([]ma.Multiaddr, len(addrs))
	for i, addr := range addrs {
		var err error
		maddrs[i], err = ma.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
	}
	return peer.AddrInfosFromP2pAddrs(maddrs...)
}

func logError(err error) {
	fmt.Println(red("Error: " + err.Error()))
}
