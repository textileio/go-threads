package main

import (
	"bufio"
	"context"
	"flag"
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
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	swarm "github.com/libp2p/go-libp2p-swarm"
	"github.com/libp2p/go-libp2p/p2p/discovery"
	"github.com/mr-tron/base58"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	sym "github.com/textileio/go-textile-core/crypto/symmetric"
	"github.com/textileio/go-textile-core/options"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
	t "github.com/textileio/go-textile-threads"
	"github.com/textileio/go-textile-threads/cbor"
	"github.com/textileio/go-textile-threads/exe/util"
	tutil "github.com/textileio/go-textile-threads/util"
	"github.com/textileio/go-textile-threads/ws"
)

var (
	ctx      context.Context
	ds       datastore.Batching
	dht      *kaddht.IpfsDHT
	api      tserv.Threadservice
	threadID thread.ID

	grey  = color.New(color.FgHiBlack).SprintFunc()
	green = color.New(color.FgHiGreen).SprintFunc()
	cyan  = color.New(color.FgHiCyan).SprintFunc()
	pink  = color.New(color.FgHiMagenta).SprintFunc()
	red   = color.New(color.FgHiRed).SprintFunc()

	cursor = green(">  ")

	log = logging.Logger("shell")
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
	api.Host().Peerstore().AddAddrs(p.ID, p.Addrs, pstore.ConnectedAddrTTL)
}

func main() {
	repo := flag.String("repo", ".threads", "repo location")
	port := flag.Int("port", 4006, "host port")
	proxyAddr := flag.String("proxy", "", "proxy server address")
	wsAddr := flag.String("ws", "", "web socket server address")
	flag.Parse()

	if err := logging.SetLogLevel("shell", "debug"); err != nil {
		panic(err)
	}

	var cancel context.CancelFunc
	var h host.Host
	ctx, cancel, ds, h, dht, api = util.Build(*repo, *port, *proxyAddr, true)

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
	mdns.RegisterNotifee(&notifee{})

	// Start a web socket server
	if *wsAddr == "" {
		*wsAddr = "0.0.0.0:8080"
	}
	wsServer := ws.NewServer(api, *wsAddr)
	defer wsServer.Close()
	if err = logging.SetLogLevel("ws", "debug"); err != nil {
		panic(err)
	}

	// Start the prompt
	fmt.Println(grey("Welcome to Threads!"))
	fmt.Println(grey("Your peer ID is ") + green(h.ID().String()))

	sub := api.Subscribe()
	go func() {
		for {
			select {
			case rec, ok := <-sub.Channel():
				if !ok {
					return
				}

				name, err := threadName(rec.ThreadID().String())
				if err != nil {
					logError(err)
					continue
				}
				event, err := cbor.EventFromRecord(ctx, api, rec.Value())
				if err != nil {
					logError(err)
					continue
				}
				key, err := api.Store().ReadKey(rec.ThreadID())
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

				clean(0)

				fmt.Println(pink(name+"> ") +
					grey(msgTime.Format(timeLayout)+" ") +
					cyan(shortID(rec.LogID())+"  ") +
					grey(m.Txt))

				fmt.Print(cursor)
			}
		}
	}()

	log.Debug("shell started")

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
	return
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
		fkb, err := base58.Decode(args[2])
		if err != nil {
			return "", err
		}
		fk, err = sym.NewKey(fkb)
		if err != nil {
			return "", err
		}
	}
	if len(args) > 3 {
		rkb, err := base58.Decode(args[3])
		if err != nil {
			return "", err
		}
		rk, err = sym.NewKey(rkb)
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
		if !tutil.CanDial(addr, api.Host().Network().(*swarm.Swarm)) {
			return "", fmt.Errorf("address is not dialable")
		}
		info, err := api.AddThread(ctx, addr, options.FollowKey(fk), options.ReadKey(rk))
		if err != nil {
			return "", err
		}
		id = info.ID
	} else {
		th, err := tutil.CreateThread(api, thread.NewIDV1(thread.Raw, 32))
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
	lg, err := tutil.GetOwnLog(api, id)
	if err != nil {
		return
	}
	ta, err := ma.NewMultiaddr("/" + t.Thread + "/" + id.String())
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
	info, err := api.Store().ThreadInfo(id)
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

	pid, err := tutil.AddPeerFromAddress(addrStr, api.Host().Peerstore())
	if err != nil {
		return
	}

	if err = api.AddFollower(ctx, id, pid); err != nil {
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
		_, err = api.AddRecord(mctx, id, body)
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
