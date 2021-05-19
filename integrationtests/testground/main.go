package main

import (
	"context"
	crand "crypto/rand"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"time"

	fuzz "github.com/google/gofuzz"
	cid "github.com/ipfs/go-cid"
	ipldcbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	crypto "github.com/libp2p/go-libp2p-core/crypto"
	libp2ppeer "github.com/libp2p/go-libp2p-peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	sync "github.com/testground/sdk-go/sync"
	"google.golang.org/grpc"

	"github.com/textileio/go-threads/cbor"
	corenet "github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/net/api"
	"github.com/textileio/go-threads/net/api/client"
	"github.com/textileio/go-threads/util"
)

var (
	netSlow = network.LinkShape{
		Latency:   time.Second,
		Jitter:    100 * time.Millisecond,
		Bandwidth: 1 << 20, // 1Mib
	}
	netLongFat = network.LinkShape{
		Latency:   time.Second,
		Bandwidth: 1 << 30, // 1Gib
	}
	netLowBW = network.LinkShape{
		Latency:   100 * time.Millisecond,
		Bandwidth: 1 << 16, // 64Kib
	}
	netMildlyCongested = network.LinkShape{
		Latency:   100 * time.Millisecond,
		Bandwidth: 1 << 30, // 1Gib
		Loss:      10,
	}
	netTerriblyCongested = network.LinkShape{
		Latency:   100 * time.Millisecond,
		Bandwidth: 1 << 30, // 1Gib
		Loss:      40,
	}

	netConditions = []struct {
		simulation string
		shape      network.LinkShape
	}{
		{"normal", network.LinkShape{}},
		{"slow", netSlow},
		{"long-fat", netLongFat},
		{"low-bw", netLowBW},
		{"mildly-congested", netMildlyCongested},
		{"terribly-congested", netTerriblyCongested},
	}

	fuzzer = fuzz.New()

	msg   func(msg string, args ...interface{})
	debug func(msg string, args ...interface{})
	fail  func(msg string, args ...interface{})
)

func main() {
	run.InvokeMap(map[string]interface{}{
		"sync-threads": run.InitializedTestCaseFn(syncThreads),
	})
}

func syncThreads(env *runtime.RunEnv, ic *run.InitContext) (err error) {
	var (
		numRecords = env.IntParam("records")
		numPeers   = env.TestInstanceCount
	)
	msg = func(msg string, args ...interface{}) {
		env.RecordMessage(msg, args...)
	}
	debug = func(msg string, args ...interface{}) {
		if env.IntParam("verbose") > 0 {
			env.RecordMessage(msg, args...)
		}
	}
	fail = func(msg string, args ...interface{}) {
		env.RecordFailure(fmt.Errorf(msg, args...))
	}

	addr := setup(env, ic)
	// starts the API server and client
	hostAddr, gRPCAddr, shutdown, err := api.CreateTestService(addr, env.IntParam("verbose") > 1)
	if err != nil {
		return err
	}
	defer shutdown()
	msg("Peer #%d, p2p listening on %v, gRPC listening on %v", ic.GlobalSeq, hostAddr, gRPCAddr)
	target, err := util.TCPAddrFromMultiAddr(gRPCAddr)
	if err != nil {
		return err
	}
	client, err := client.NewClient(target, grpc.WithInsecure(), grpc.WithPerRPCCredentials(thread.Credentials{}))
	if err != nil {
		return err
	}
	defer client.Close()

	doTest := func(round string, topic *sync.Topic, chThreadToBeJoin chan SharedInfo) error {
		start := time.Now()
		ctx, _ := context.WithTimeout(context.Background(), time.Minute)
		shared := <-chThreadToBeJoin
		thr, err := joinThread(ctx, client, &shared)
		if err != nil {
			return fmt.Errorf("failed to join thread: %w", err)
		}
		msg("Joined thread")

		doneStep1 := sync.State("done-" + round + "-step1")
		var logRecords map[libp2ppeer.ID][]corenet.Record
		go func() {
			logRecords = thr.waitForRecords(ctx, numRecords*env.TestInstanceCount)
			for logID, records := range logRecords {
				if len(records) < numRecords {
					fail("Expect %d records from log %v, got %d", numRecords, logID, len(records))
					return
				}
			}
			env.R().RecordPoint("round-"+round+"-step-1-elapsed-seconds", time.Since(start).Seconds())
			msg("Peer #%d done round %s step 1", ic.GlobalSeq, round)
			ic.SyncClient.MustSignalAndWait(ctx, doneStep1, env.TestInstanceCount)
		}()

		prev := cid.Undef
		for i := 0; i < numRecords; i++ {
			rec, err := thr.addRecord(ctx, prev)
			if err != nil {
				return err
			}
			prev = rec.Cid()
		}
		msg("Peer #%d shoot out %d records", ic.GlobalSeq, numRecords)
		// wait until all peers get correct results
		<-ic.SyncClient.MustBarrier(ctx, doneStep1, numPeers).C

		doneStep2 := sync.State("done-" + round + "-step2")
		start = time.Now()
		go func() {
			_ = thr.waitForRecords(ctx, numRecords*env.TestInstanceCount*env.TestInstanceCount)
			env.R().RecordPoint("round-"+round+"-step-2-elapsed-seconds", time.Since(start).Seconds())
			msg("Peer #%d done round %s step 2", ic.GlobalSeq, round)
			ic.SyncClient.MustSignalAndWait(ctx, doneStep2, env.TestInstanceCount)
		}()
		// now add records pointing to each other's previous records
		for _, records := range logRecords {
			for _, rec := range records {
				_, err := thr.addRecord(ctx, rec.Cid())
				if err != nil {
					return fmt.Errorf("Error creating new record: %w", err)
				}
			}
		}
		<-ic.SyncClient.MustBarrier(ctx, doneStep2, numPeers).C
		return nil
	}

	for _, pair := range netConditions {
		round := pair.simulation
		ctx, _ := context.WithTimeout(context.Background(), time.Minute)
		chThreadToBeJoin := make(chan SharedInfo, 1)
		topic := sync.NewTopic("thread-"+round, SharedInfo{})
		self := ic.GlobalSeq
		if self == 1 {
			msg("################### Round %s network ###################", round)
			// peer 1: reconfigure network, create the thread shared by all peers, then signal others to continue
			ic.NetClient.MustConfigureNetwork(ctx, &network.Config{
				Network:       "default",
				Enable:        true,
				Default:       pair.shape,
				CallbackState: sync.State("network-configured-" + round),
				RoutingPolicy: network.AllowAll,
			})
			msg("Done configuring network")
			// create this "root" thread and broadcast, then each
			// instance (include this one itself) creates its own
			// log in this thread.
			rootThread, err := createThread(ctx, client)
			if err != nil {
				msg("Failed to create the thread: %v", err)
				return err
			}
			msg("Created thread")
			ic.SyncClient.MustPublishSubscribe(ctx,
				topic,
				rootThread.Sharable(),
				chThreadToBeJoin)
		} else {
			ic.SyncClient.MustSubscribe(ctx, topic, chThreadToBeJoin)
		}

		// make sure all peers start the test at the same time
		ic.SyncClient.MustSignalAndWait(ctx, sync.State("ready-"+round), env.TestInstanceCount)
		msg("Peer #%d starting round %s", self, round)
		if err := doTest(round, topic, chThreadToBeJoin); err != nil {
			msg("################### Peer #%d round %s failed: %v ###################", self, round, err)
			return err
		}
	}
	return nil
}

func setup(env *runtime.RunEnv, ic *run.InitContext) (hostAddr string) {
	logLevel := logging.LevelError
	switch env.IntParam("verbose") {
	case 1:
		logLevel = logging.LevelInfo
	case 2:
		logLevel = logging.LevelDebug
	default:
	}
	logging.SetupLogging(logging.Config{
		Format: logging.ColorizedOutput,
		Stdout: true,
		Level:  logLevel,
	})
	if ip := ic.NetClient.MustGetDataNetworkIP().String(); ip != "127.0.0.1" {
		// listen on public IP when running in container, so the numPeers can
		// communicate with each other
		return fmt.Sprintf("/ip4/%s/tcp/8765", ip)
	} else {
		go func() {
			l, _ := net.Listen("tcp", ":")
			env.RecordMessage("starting pprof at %s/debug/pprof", l.Addr().String())
			_ = http.Serve(l, nil)
		}()
		// running locally, use random local port to avoid port conflict
		return ""
	}
}

// info shared among peers via testground pubsub
type SharedInfo struct {
	Addr      string // multiaddr.Multiaddr
	ThreadKey string // thread.Key
}

type ThreadWithKeys struct {
	thread.Info
	identity thread.Identity
	logSk    crypto.PrivKey
	logPk    crypto.PubKey
	cli      *client.Client
}

func createThread(ctx context.Context, cli *client.Client) (thr *ThreadWithKeys, err error) {
	// Create a thread, keeping read key and log private key on the client
	id := thread.NewIDV1(thread.Raw, 32)
	tk := thread.NewRandomKey()
	logSk, logPk, err := crypto.GenerateEd25519Key(crand.Reader)
	if err != nil {
		return nil, err
	}
	sk, _, err := crypto.GenerateEd25519Key(crand.Reader)
	if err != nil {
		return nil, err
	}
	identity := thread.NewLibp2pIdentity(sk)
	tok, err := cli.GetToken(ctx, identity)
	if err != nil {
		return nil, err
	}

	info, err := cli.CreateThread(ctx, id,
		corenet.WithThreadKey(tk),
		corenet.WithLogKey(logPk),
		corenet.WithNewThreadToken(tok))
	if err != nil {
		return nil, err
	}
	return &ThreadWithKeys{info, identity, logSk, logPk, cli}, nil
}

// joinThread joins to a thread created by another peer. It allows reading records created by the peer, also creates its own log to the thread.
func joinThread(ctx context.Context, cli *client.Client, shared *SharedInfo) (*ThreadWithKeys, error) {
	addr, err := multiaddr.NewMultiaddr(shared.Addr)
	if err != nil {
		return nil, err
	}
	key, err := thread.KeyFromString(shared.ThreadKey)
	if err != nil {
		return nil, err
	}
	logSk, logPk, err := crypto.GenerateEd25519Key(crand.Reader)
	if err != nil {
		return nil, err
	}
	sk, _, err := crypto.GenerateEd25519Key(crand.Reader)
	if err != nil {
		return nil, err
	}
	identity := thread.NewLibp2pIdentity(sk)
	tok, err := cli.GetToken(ctx, identity)
	if err != nil {
		return nil, err
	}
	info, err := cli.AddThread(ctx, addr, corenet.WithThreadKey(key), corenet.WithLogKey(logSk), corenet.WithNewThreadToken(tok))
	if err != nil {
		return nil, err
	}
	return &ThreadWithKeys{info, identity, logSk, logPk, cli}, nil
}

func emptyThread(cli *client.Client) (thr *ThreadWithKeys) {
	return &ThreadWithKeys{cli: cli}
}

func (t *ThreadWithKeys) Sharable() *SharedInfo {
	return &SharedInfo{t.Addrs[0].String(), t.Key.String()}
}

// waitForRecords blocks until it receives nRecords, then return them classified by log ID
func (t *ThreadWithKeys) waitForRecords(ctx context.Context, nRecords int) (logRecords map[libp2ppeer.ID][]corenet.Record) {
	logRecords = make(map[libp2ppeer.ID][]corenet.Record)
	ch, err := t.cli.Subscribe(ctx, corenet.WithSubFilter(t.Info.ID))
	if err != nil {
		msg("Error subscribing thread: %v", err)
		return
	}
	debug("Subscribed to thread")
	totalRecords := 0
	for record := range ch {
		totalRecords++
		logID := record.LogID()
		records := logRecords[logID]
		records = append(records, record.Value())
		debug("Records of log %v so far: %d", logID, len(records))
		logRecords[logID] = records
		if totalRecords >= nRecords {
			msg("Got all %d records", nRecords)
			return
		}
	}
	return
}

func (t *ThreadWithKeys) addRecord(ctx context.Context, prev cid.Cid) (rec corenet.Record, err error) {
	obj := make(map[string][]byte)
	fuzzer.Fuzz(&obj)
	body, err := ipldcbor.WrapObject(obj, multihash.SHA2_256, -1)
	event, err := cbor.CreateEvent(ctx, nil, body, t.Key.Read())
	if err != nil {
		return nil, err
	}
	rec, err = cbor.CreateRecord(ctx, nil, cbor.CreateRecordConfig{
		Block:      event,
		Prev:       prev,
		Key:        t.logSk,
		PubKey:     t.identity.GetPublic(),
		ServiceKey: t.Key.Service(),
	})
	if err != nil {
		return nil, err
	}
	logID, err := libp2ppeer.IDFromPublicKey(t.logPk)
	if err != nil {
		return nil, err
	}

	if err = t.cli.AddRecord(ctx, t.ID, logID, rec); err != nil {
		return nil, err
	}
	return rec, nil
}
