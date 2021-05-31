// This is the test program to be run by testground. Here's what it does:
// First, configure testground to simulate different network scenarios.
// In each scenario:
// |-- have one test instance creating the thread and broadcast.
// |-- each test instance join the thread, then:
//     |-- 1. create a few records goverend by the "records" test param.
//     |-- 2. make sure records created by each instance are synced to all instances.
//     |-- 3. for each of these records, create a new record use it as the "prev". This effectively creates a network of records on which each log has records referencing some other log's records.
//     |-- 4. make sure to receive the new records created by all instances in above step, and be able to traverse all the way back via the "prev" pointer.
//
// It also allows configuring a few "early-stop" and "late-start" instances, which stop / start before step 3 respectively, to simulate the situation that after some peers disconnect from the network, their records are still accessible to newly joined peers, via those who have a copy of them.

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
	"github.com/libp2p/go-libp2p-peer"
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

	simulatedNetworks = []struct {
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

	// handy helpers which get initialized at the very beginning of the test.
	msg   func(msg string, args ...interface{})
	debug func(msg string, args ...interface{})
	fail  func(msg string, args ...interface{})
)

func main() {
	run.InvokeMap(map[string]interface{}{
		"sync-threads": run.InitializedTestCaseFn(testSyncThreads),
	})
}

func testSyncThreads(env *runtime.RunEnv, ic *run.InitContext) (err error) {
	msg = func(msg string, args ...interface{}) {
		env.RecordMessage(msg, args...)
	}
	debug = func(msg string, args ...interface{}) {
		if env.IntParam("verbose") > 1 {
			env.RecordMessage(msg, args...)
		}
	}
	fail = func(msg string, args ...interface{}) {
		env.RecordFailure(fmt.Errorf(msg, args...))
	}
	setup(env, ic)
	for i, pair := range simulatedNetworks {
		round := pair.simulation
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		if ic.GlobalSeq == 1 {
			msg("################### Round %s network ###################", round)
			ic.NetClient.MustConfigureNetwork(ctx, &network.Config{
				Network:        "default",
				Enable:         true,
				Default:        pair.shape,
				CallbackState:  sync.State("network-configured-" + round),
				CallbackTarget: 1,
				RoutingPolicy:  network.AllowAll,
			})
			msg("Done configuring %s network", round)
		}

		desiredAddr := ""
		if env.TestSidecar {
			// listen on non-local interface when running in container, so the peers can communicate with each other
			ip := ic.NetClient.MustGetDataNetworkIP().String()
			desiredAddr = fmt.Sprintf("/ip4/%s/tcp/%d", ip, 3000+i)
		}
		if err := testRound(ctx, env, ic, round, desiredAddr); err != nil {
			msg("################### Peer #%d with %s network failed: %v ###################", ic.GlobalSeq, round, err)

			cancel()
			return err
		}
		cancel()
	}
	return nil
}

func testRound(ctx context.Context, env *runtime.RunEnv, ic *run.InitContext, round string, desiredAddr string) error {
	var cli *client.Client
	var stop func()
	isLateStart := env.BooleanParam("late-start")
	isEarlyStop := env.BooleanParam("early-stop")
	if !isLateStart {
		var err error
		cli, stop, err = startClient(desiredAddr, env, ic)
		if err != nil {
			return err
		}
	}

	chThreadToJoin := make(chan *SharedInfo, 1)
	topic := sync.NewTopic("thread-"+round, &SharedInfo{})
	// choose a single peer to create the thread and broadcast, then each peer (include this one itself) creates its own log on this thread.
	if !isLateStart && !isEarlyStop && ic.GroupSeq == 1 {
		thr, err := createThread(ctx, cli)
		if err != nil {
			msg("Failed to create the thread: %v", err)
			return err
		}
		msg("Created thread")
		ic.SyncClient.MustPublishSubscribe(ctx,
			topic,
			thr.Sharable(),
			chThreadToJoin)
		msg("Published thread")
	} else {
		ic.SyncClient.MustSubscribe(ctx, topic, chThreadToJoin)
	}

	// peers each send their own number and have a consensus on the correct total
	publishAndGetTotal := func(topic *sync.Topic, myNum int) (total int) {
		ch := make(chan int)
		ic.SyncClient.MustPublishSubscribe(ctx, topic, myNum, ch)
		for i := 0; i < env.TestInstanceCount; i++ {
			total += <-ch
		}
		return
	}
	recordsToSend := env.IntParam("records")
	meLive := 1
	if isLateStart {
		recordsToSend = 0
		meLive = 0
	}
	recordsToReceive := publishAndGetTotal(sync.NewTopic("recordsToReceive-"+round+"-step-1", 0), recordsToSend)
	livePeers := publishAndGetTotal(sync.NewTopic("livePeers-"+round+"-step-1", 0), meLive)

	var receivedRecords []corenet.Record
	var thr *threadWithKeys
	if !isLateStart {
		var err error
		thr, err = joinThread(ctx, cli, <-chThreadToJoin)
		if err != nil {
			return fmt.Errorf("failed to join thread: %w", err)
		}
		msg("Joined thread")

		ic.SyncClient.MustSignalAndWait(ctx, sync.State("ready-"+round+"-step1"), livePeers)
		start := time.Now()
		doneStep1 := sync.State("done-" + round + "-step1")
		go func() {
			receivedRecords = thr.WaitForRecords(ctx, recordsToReceive)
			env.R().RecordPoint("round-"+round+"-step-1-elapsed-seconds", time.Since(start).Seconds())
			msg("Peer #%d done %s network step 1, received %d records", ic.GlobalSeq, round, len(receivedRecords))
			ic.SyncClient.MustSignalAndWait(ctx, doneStep1, livePeers)
		}()

		for i, prev := 0, cid.Undef; i < recordsToSend; i++ {
			rec, err := thr.AddRecord(ctx, prev)
			if err != nil {
				return err
			}
			debug("Added record #%d: %v", i+1, rec)
			prev = rec.Cid()
		}
		msg("Peer #%d added %d records", ic.GlobalSeq, recordsToSend)
		// wait until all live peers get correct results
		<-ic.SyncClient.MustBarrier(ctx, doneStep1, livePeers).C
	}

	recordsToSend = recordsToReceive
	meLive = 1
	if isEarlyStop {
		meLive = 0
		recordsToSend = 0
	}
	livePeers = publishAndGetTotal(sync.NewTopic("livePeers-"+round+"-step-2", 0), meLive)
	recordsToReceive = publishAndGetTotal(sync.NewTopic("recordsToReceive-"+round+"-step-2", 0), recordsToSend)
	if isEarlyStop {
		stop()
		return nil
	}
	if cli == nil {
		// these are late-start ones
		var err error
		cli, stop, err = startClient(desiredAddr, env, ic)
		if err != nil {
			return err
		}

		thr, err = joinThread(ctx, cli, <-chThreadToJoin)
		if err != nil {
			return fmt.Errorf("failed to join thread: %w", err)
		}
		msg("Joined thread")
	}

	ic.SyncClient.MustSignalAndWait(ctx, sync.State("ready-"+round+"-step2"), livePeers)
	start := time.Now()
	doneState := sync.State("done-" + round + "-step2")
	go func() {
		var err error
		allRecords := thr.WaitForRecords(ctx, recordsToReceive)
		for _, rec := range allRecords {
			// trace back to all previous records, include the ones created by inactive peers
			for rec.PrevID() != cid.Undef {
				rec, err = cli.GetRecord(ctx, thr.ID, rec.PrevID())
				if err != nil {
					fail("Error getting record: %v", err)
					return
				}
			}
		}
		env.R().RecordPoint("round-"+round+"-step-2-elapsed-seconds", time.Since(start).Seconds())
		msg("Peer #%d done %s network step 2, received %d records", ic.GlobalSeq, round, len(allRecords))
		ic.SyncClient.MustSignalAndWait(ctx, doneState, livePeers)
	}()
	// now create the same amount of records as received, use the received ones as prev.
	msg("Adding %d records", len(receivedRecords))
	for i, rec := range receivedRecords {
		rec2, err := thr.AddRecord(ctx, rec.Cid())
		if err != nil {
			return fmt.Errorf("Error creating new record: %w", err)
		}
		debug("Added record #%d: %v", i+1, rec2)
	}

	<-ic.SyncClient.MustBarrier(ctx, doneState, livePeers).C
	stop()
	return nil
}

func setup(env *runtime.RunEnv, ic *run.InitContext) {
	logLevel := logging.LevelError
	switch env.IntParam("verbose") {
	case 1:
		logLevel = logging.LevelWarn
	case 2:
		logLevel = logging.LevelInfo
	case 3:
		logLevel = logging.LevelDebug
	default:
	}
	logging.SetupLogging(logging.Config{
		Format: logging.ColorizedOutput,
		Stdout: true,
		Level:  logLevel,
	})
	if !env.TestSidecar {
		// starts pprof when running local:exec, in which case the URL is directly accessible
		go func() {
			l, _ := net.Listen("tcp", ":")
			env.RecordMessage("starting pprof at http://%s/debug/pprof", l.Addr().String())
			_ = http.Serve(l, nil)
		}()
	}
}

func startClient(desiredAddr string, env *runtime.RunEnv, ic *run.InitContext) (*client.Client, func(), error) {
	// starts the API server and client
	hostAddr, gRPCAddr, shutdown, err := api.CreateTestService(desiredAddr, env.IntParam("verbose") > 1)
	if err != nil {
		return nil, func() {}, err
	}
	msg("Peer #%d, p2p listening on %v, gRPC listening on %v", ic.GlobalSeq, hostAddr, gRPCAddr)
	target, err := util.TCPAddrFromMultiAddr(gRPCAddr)
	if err != nil {
		return nil, shutdown, err
	}
	cli, err := client.NewClient(target, grpc.WithInsecure(), grpc.WithPerRPCCredentials(thread.Credentials{}))
	if err != nil {
		return nil, shutdown, err
	}
	return cli, func() {
		cli.Close()
		shutdown()
	}, nil
}

// SharedInfo includes info shared among peers via testground pubsub
type SharedInfo struct {
	Addr      string // multiaddr.Multiaddr
	ThreadKey string // thread.Key
}

type threadWithKeys struct {
	thread.Info
	identity    thread.Identity
	logSk       crypto.PrivKey
	logPk       crypto.PubKey
	cli         *client.Client
	subscribeCh <-chan corenet.ThreadRecord
	// to deduplicate records when subscribing
	seenRecords map[cid.Cid]bool
}

func createThread(ctx context.Context, cli *client.Client) (thr *threadWithKeys, err error) {
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
	ch, err := cli.Subscribe(ctx, corenet.WithSubFilter(info.ID))
	if err != nil {
		return nil, err
	}
	return &threadWithKeys{info, identity, logSk, logPk, cli, ch, make(map[cid.Cid]bool)}, nil
}

// joinThread joins to a thread created by another peer. It allows reading records created by the peer, also creates its own log to the thread.
func joinThread(ctx context.Context, cli *client.Client, shared *SharedInfo) (*threadWithKeys, error) {
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
	ch, err := cli.Subscribe(ctx, corenet.WithSubFilter(info.ID))
	if err != nil {
		return nil, err
	}
	return &threadWithKeys{info, identity, logSk, logPk, cli, ch, make(map[cid.Cid]bool)}, nil
}

func (t *threadWithKeys) Sharable() *SharedInfo {
	return &SharedInfo{t.Addrs[0].String(), t.Key.String()}
}

// WaitForRecords blocks until it receives nRecords, then return them
func (t *threadWithKeys) WaitForRecords(ctx context.Context, nRecords int) (records []corenet.Record) {
	msg("Waiting for %d unique records", nRecords)
	total := 0
	for record := range t.subscribeCh {
		rec := record.Value()
		if _, exists := t.seenRecords[rec.Cid()]; exists {
			continue
		}
		t.seenRecords[rec.Cid()] = true
		total++
		debug("Got record #%d: %v", total, rec)
		records = append(records, record.Value())
		if total == nRecords {
			msg("Got all %d records", nRecords)
			return
		}
	}
	return
}

func (t *threadWithKeys) AddRecord(ctx context.Context, prev cid.Cid) (rec corenet.Record, err error) {
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
	logID, err := peer.IDFromPublicKey(t.logPk)
	if err != nil {
		return nil, err
	}

	if err = t.cli.AddRecord(ctx, t.ID, logID, rec); err != nil {
		return nil, err
	}
	return rec, nil
}
