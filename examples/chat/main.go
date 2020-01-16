package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	ipfslite "github.com/hsanjuan/ipfs-lite"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	cbornode "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/peer"
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	swarm "github.com/libp2p/go-libp2p-swarm"
	"github.com/libp2p/go-libp2p/p2p/discovery"
	"github.com/mr-tron/base58"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	"github.com/textileio/go-threads/api"
	"github.com/textileio/go-threads/cbor"
	core "github.com/textileio/go-threads/core/service"
	"github.com/textileio/go-threads/core/thread"
	sym "github.com/textileio/go-threads/crypto/symmetric"
	serviceapi "github.com/textileio/go-threads/service/api"
	store "github.com/textileio/go-threads/store"
	util "github.com/textileio/go-threads/util"
)

var (
	ctx      context.Context
	ds       datastore.Batching
	ts       store.ServiceBoostrapper
	threadID thread.ID

	grey  = color.New(color.FgHiBlack).SprintFunc()
	green = color.New(color.FgHiGreen).SprintFunc()
	cyan  = color.New(color.FgHiCyan).SprintFunc()
	pink  = color.New(color.FgHiMagenta).SprintFunc()
	red   = color.New(color.FgHiRed).SprintFunc()

	cursor = green(">  ")

	log = logging.Logger("chat")
)

const (
	msgTimeout = time.Second * 10
	timeLayout = "03:04:05 PM"
)

func init() {
	cbornode.RegisterCborType(msg{})
}

type msg struct {
	Txt string
}

type notifee struct{}

func (n *notifee) HandlePeerFound(p peer.AddrInfo) {
	ts.Host().Peerstore().AddAddrs(p.ID, p.Addrs, pstore.ConnectedAddrTTL)
}

func main() {
	repo := flag.String("repo", ".threads", "repo location")
	hostAddrStr := flag.String("hostAddr", "/ip4/0.0.0.0/tcp/4006", "Threads host bind address")
	serviceApiAddrStr := flag.String("serviceApiAddr", "/ip4/0.0.0.0/tcp/5006", "Threads service API bind address")
	serviceApiProxyAddrStr := flag.String("serviceApiProxyAddr", "/ip4/0.0.0.0/tcp/5007", "Threads service API gRPC proxy bind address")
	apiAddrStr := flag.String("apiAddr", "/ip4/127.0.0.1/tcp/6006", "API bind address")
	apiProxyAddrStr := flag.String("apiProxyAddr", "/ip4/127.0.0.1/tcp/7006", "API gRPC proxy bind address")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	hostAddr, err := ma.NewMultiaddr(*hostAddrStr)
	if err != nil {
		log.Fatal(err)
	}
	serviceApiAddr, err := ma.NewMultiaddr(*serviceApiAddrStr)
	if err != nil {
		log.Fatal(err)
	}
	serviceApiProxyAddr, err := ma.NewMultiaddr(*serviceApiProxyAddrStr)
	if err != nil {
		log.Fatal(err)
	}
	apiAddr, err := ma.NewMultiaddr(*apiAddrStr)
	if err != nil {
		log.Fatal(err)
	}
	apiProxyAddr, err := ma.NewMultiaddr(*apiProxyAddrStr)
	if err != nil {
		log.Fatal(err)
	}

	util.SetupDefaultLoggingConfig(*repo)
	if *debug {
		if err := logging.SetLogLevel("chat", "debug"); err != nil {
			log.Fatal(err)
		}
	}

	chatPath := filepath.Join(*repo, "chat")
	if err = os.MkdirAll(chatPath, os.ModePerm); err != nil {
		log.Fatal(err)
	}
	ds, err = ipfslite.BadgerDatastore(chatPath)
	if err != nil {
		log.Fatal(err)
	}

	ts, err = store.DefaultService(
		*repo,
		store.WithServiceHostAddr(hostAddr),
		store.WithServiceDebug(*debug))
	if err != nil {
		log.Fatal(err)
	}
	defer ts.Close()
	ts.Bootstrap(util.DefaultBoostrapPeers())

	server, err := api.NewServer(context.Background(), ts, api.Config{
		RepoPath:  *repo,
		Addr:      apiAddr,
		ProxyAddr: apiProxyAddr,
		Debug:     *debug,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()

	serviceServer, err := serviceapi.NewServer(context.Background(), ts, serviceapi.Config{
		Addr:      serviceApiAddr,
		ProxyAddr: serviceApiProxyAddr,
		Debug:     *debug,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer serviceServer.Close()

	// Build a MDNS service
	ctx = context.Background()
	mdns, err := discovery.NewMdnsService(ctx, ts.Host(), time.Second, "")
	if err != nil {
		log.Fatal(err)
	}
	defer mdns.Close()
	mdns.RegisterNotifee(&notifee{})

	// Start the prompt
	fmt.Println(grey("Welcome to Threads!"))
	fmt.Println(grey("Your peer ID is ") + green(ts.Host().ID().String()))

	sub, err := ts.Subscribe(ctx)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		for rec := range sub {
			name, err := threadName(rec.ThreadID().String())
			if err != nil {
				logError(err)
				continue
			}
			key, err := ts.Store().ReadKey(rec.ThreadID())
			if err != nil {
				logError(err)
				continue
			}
			if key == nil {
				continue // just following, we don't have the read key
			}
			event, err := cbor.EventFromRecord(ctx, ts, rec.Value())
			if err != nil {
				logError(err)
				continue
			}
			node, err := event.GetBody(ctx, ts, key)
			if err != nil {
				continue // Not for us
			}
			m := new(msg)
			err = cbornode.DecodeInto(node.RawData(), m)
			if err != nil {
				continue // Not one of our messages
			}
			header, err := event.GetHeader(ctx, ts, key)
			if err != nil {
				logError(err)
				continue
			}
			msgTime, err := header.Time()
			if err != nil {
				logError(err)
				continue
			}

			clean(0)

			fmt.Println(pink(name+"> ") +
				grey(msgTime.Format(timeLayout)+" ") +
				cyan(shortID(rec.LogID())+"  ") +
				grey(m.Txt))

			fmt.Print(cursor)
		}
	}()

	log.Debug("chat started")

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(cursor)
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
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
	if lineCnt == 0 {
		_, _ = buf.WriteString("\033[2K")
		_, _ = buf.WriteString("\r")
	} else {
		for i := 0; i < lineCnt; i++ {
			_, _ = io.WriteString(buf, "\033[2K\r\033[A")
		}
		_, _ = io.WriteString(buf, "\033[2K\r")
	}
	_ = buf.Flush()
}

func handleLine(line string) (out string, err error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	if strings.HasPrefix(line, ":") {
		parts := strings.SplitN(line, " ", 2)
		cmds := strings.Split(parts[0], ":")
		cmds = cmds[1:]
		switch cmds[0] {
		case "help":
			return cmdCmd()
		case "address":
			if threadID.Defined() {
				return threadAddressCmd(threadID)
			} else {
				return addressCmd()
			}
		case "keys":
			if !threadID.Defined() {
				err = fmt.Errorf("enter a thread with `:enter` or specify thread name with :<name>")
				return
			}
			return threadKeysCmd(threadID)
		case "threads":
			return threadsCmd()
		case "enter":
			if len(parts) == 1 {
				err = fmt.Errorf("missing thread name")
				return
			}
			return enterCmd(parts[1])
		case "exit":
			threadID = thread.Undef
			cursor = green(">  ")
			return
		case "add":
			if len(parts) == 1 {
				err = fmt.Errorf("missing thread name")
				return
			}
			args := strings.Split(parts[1], " ")
			return addCmd(args)
		case "add-follower":
			if !threadID.Defined() {
				err = fmt.Errorf("enter a thread with `:enter` or specify thread name with :<name>")
				return
			}
			if len(parts) == 1 {
				err = fmt.Errorf("missing peer address")
				return
			}
			return addFollowerCmd(threadID, parts[1])
		case "":
			err = fmt.Errorf("missing command")
			return
		default:
			var input string
			if len(parts) > 1 {
				input = parts[1]
			}
			return threadCmd(cmds, input)
		}
	}

	if threadID.Defined() {
		err = sendMessage(threadID, line)
	} else {
		err = fmt.Errorf("enter a thread with `:enter` or specify thread name with :<name>")
	}
	return
}

func cmdCmd() (out string, err error) {
	out = pink(":help  ") + grey("Show available commands.\n")
	out += pink(":address  ") + grey("Show the host or active thread addresses.\n")
	out += pink(":threads  ") + grey("Show threads.\n")
	out += pink(":add <name>  ") + grey("Add a new thread with name.\n")
	out += pink(":add <name> <address> <follow-key> <read-key>  ") +
		grey("Add an existing thread with name at address using a base58-encoded follow and read key.\n")
	out += pink(":<name> <message>  ") + grey("Send a message to thread with name.\n")
	out += pink(":<name>:address  ") + grey("Show thread address.\n")
	out += pink(":<name>:keys  ") + grey("Show thread keys.\n")
	out += pink(":<name>:add-follower <address>  ") + grey("Add a follower at address.\n")
	out += pink(":enter <name>  ") + grey("Enter thread with name.\n")
	out += pink(":exit  ") + grey("Exit the active thread.\n")
	out += pink(":keys  ") + grey("Show the active thread's keys.\n")
	out += pink(":add-follower <address>  ") + grey("Add a follower at address to active thread.\n")
	out += pink("<message>  ") + grey("Send a message to the active thread.")
	return
}

func addressCmd() (out string, err error) {
	pro := ma.ProtocolWithCode(ma.P_P2P).Name
	addr, err := ma.NewMultiaddr("/" + pro + "/" + ts.Host().ID().String())
	if err != nil {
		return
	}
	addrs := ts.Host().Addrs()
	for i, a := range addrs {
		a = a.Encapsulate(addr)
		out += a.String()
		if i != len(addrs)-1 {
			out += "\n"
		}
	}
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
		out += pink(name) + grey(" ("+id.String()+")")
		if i != len(all)-1 {
			out += "\n"
		}
	}
	return
}

func enterCmd(name string) (out string, err error) {
	idv, err := ds.Get(datastore.NewKey("/names/" + name))
	if err != nil {
		err = fmt.Errorf("thread not found")
		return
	}
	threadID, err = thread.Cast(idv)
	if err != nil {
		return
	}

	cursor = green(name + "> ")
	return
}

func addCmd(args []string) (out string, err error) {
	name := args[0]
	var addr ma.Multiaddr
	if len(args) > 1 {
		addr, err = ma.NewMultiaddr(args[1])
		if err != nil {
			return
		}
	}

	var fk, rk *sym.Key
	if len(args) > 2 {
		fk, err = util.DecodeKey(args[2])
		if err != nil {
			return "", err
		}
	}
	if len(args) > 3 {
		rk, err = util.DecodeKey(args[3])
		if err != nil {
			return "", err
		}
	}

	x, err := ds.Has(datastore.NewKey("/names/" + name))
	if err != nil {
		return
	}
	if x {
		err = fmt.Errorf("thread name exists")
		return
	}

	var id thread.ID
	if addr != nil {
		if !util.CanDial(addr, ts.Host().Network().(*swarm.Swarm)) {
			return "", fmt.Errorf("address is not dialable")
		}
		info, err := ts.AddThread(ctx, addr, core.FollowKey(fk), core.ReadKey(rk))
		if err != nil {
			return "", err
		}
		id = info.ID
	} else {
		th, err := util.CreateThread(ts, thread.NewIDV1(thread.Raw, 32))
		if err != nil {
			return "", err
		}
		id = th.ID
	}
	if err = ds.Put(datastore.NewKey("/names/"+name), id.Bytes()); err != nil {
		return
	}

	if err = sendMessage(id, "👋"); err != nil {
		return
	}

	out = fmt.Sprintf("Added thread %s", id.String())
	return
}

func threadCmd(cmds []string, input string) (out string, err error) {
	name := cmds[0]

	idv, err := ds.Get(datastore.NewKey("/names/" + name))
	if err != nil {
		err = fmt.Errorf("thread not found")
		return
	}
	id, err := thread.Cast(idv)
	if err != nil {
		return
	}

	if len(cmds) > 1 {
		switch cmds[1] {
		case "address":
			return threadAddressCmd(id)
		case "keys":
			return threadKeysCmd(id)
		case "add-follower":
			return addFollowerCmd(id, input)
		default:
			err = fmt.Errorf("unknown command: %s", cmds[1])
			return
		}
	}

	err = sendMessage(id, input)
	return
}

func threadAddressCmd(id thread.ID) (out string, err error) {
	lg, err := util.GetOwnLog(ts, id)
	if err != nil {
		return
	}
	ta, err := ma.NewMultiaddr("/" + thread.Name + "/" + id.String())
	if err != nil {
		return
	}

	var addrs []ma.Multiaddr
	for _, la := range lg.Addrs {
		p2p, err := la.ValueForProtocol(ma.P_P2P)
		if err != nil {
			return "", err
		}
		pid, err := peer.Decode(p2p)
		if err != nil {
			return "", err
		}

		var paddrs []ma.Multiaddr
		if pid.String() == ts.Host().ID().String() {
			paddrs = ts.Host().Addrs()
		} else {
			paddrs = ts.Host().Peerstore().Addrs(pid)
		}
		for _, pa := range paddrs {
			addrs = append(addrs, pa.Encapsulate(la).Encapsulate(ta))
		}
	}

	if len(addrs) == 0 {
		err = fmt.Errorf("thread is empty")
		return
	}

	for i, a := range addrs {
		out += a.String()
		if i != len(addrs)-1 {
			out += "\n"
		}
	}

	return
}

func threadKeysCmd(id thread.ID) (out string, err error) {
	info, err := ts.Store().ThreadInfo(id)
	if err != nil {
		return
	}

	if info.FollowKey != nil {
		out += grey(base58.Encode(info.FollowKey.Bytes())) + cyan(" (follow-key)")
	}
	if info.ReadKey != nil {
		out += "\n"
		out += grey(base58.Encode(info.ReadKey.Bytes())) + red(" (read-key)")
	}

	return
}

func addFollowerCmd(id thread.ID, addrStr string) (out string, err error) {
	if addrStr == "" {
		err = fmt.Errorf("enter a peer address")
		return
	}

	pid, err := util.AddPeerFromAddress(addrStr, ts.Host().Peerstore())
	if err != nil {
		return
	}

	if err = ts.AddFollower(ctx, id, pid); err != nil {
		return
	}
	return "Added follower " + pid.String(), nil
}

func sendMessage(id thread.ID, txt string) error {
	if strings.TrimSpace(txt) == "" {
		return fmt.Errorf("missing message")
	}

	body, err := cbornode.WrapObject(&msg{Txt: txt}, mh.SHA2_256, -1)
	if err != nil {
		return err
	}
	go func() {
		mctx, cancel := context.WithTimeout(ctx, msgTimeout)
		defer cancel()
		_, err = ts.AddRecord(mctx, id, body)
		if err != nil {
			log.Errorf("error writing message: %s", err)
		}
	}()
	return nil
}

func threadName(id string) (name string, err error) {
	q, err := ds.Query(query.Query{Prefix: "/names"})
	if err != nil {
		return
	}
	all, err := q.Rest()
	if err != nil {
		return
	}

	for _, e := range all {
		i, err := thread.Cast(e.Value)
		if err != nil {
			return "", err
		}
		if i.String() == id {
			return e.Key[strings.LastIndex(e.Key, "/")+1:], nil
		}
	}
	return
}

func shortID(id peer.ID) string {
	l := id.String()
	return l[len(l)-7:]
}

func logError(err error) {
	fmt.Println(red("Error: " + err.Error()))
}
