package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"time"

	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	tgsync "github.com/testground/sdk-go/sync"

	ipldcbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
	corenet "github.com/textileio/go-threads/core/net"
	corethread "github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/net/api"
	"github.com/textileio/go-threads/net/api/client"
	"github.com/textileio/go-threads/util"
	"google.golang.org/grpc"
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
	msg := func(msg string, args ...interface{}) {
		env.RecordMessage(msg, args...)
	}
	debug := func(msg string, args ...interface{}) {
		if env.BooleanParam("debug") {
			env.RecordMessage(msg, args...)
		}
	}

	addr := setup(env, ic)
	// starts the API server and client
	hostAddr, gRPCAddr, shutdown, err := api.CreateTestService(addr, env.BooleanParam("debug"))
	if err != nil {
		return err
	}
	defer shutdown()
	msg("Peer #%d, p2p started listening on %v", ic.GlobalSeq, hostAddr)
	target, err := util.TCPAddrFromMultiAddr(gRPCAddr)
	if err != nil {
		return err
	}
	client, err := client.NewClient(target, grpc.WithInsecure(), grpc.WithPerRPCCredentials(corethread.Credentials{}))
	if err != nil {
		return err
	}
	defer client.Close()

	doTest := func(round string) error {
		start := time.Now()
		doneState := tgsync.State("done-" + round)
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		theThread, err := client.CreateThread(ctx, corethread.NewIDV1(corethread.Raw, 32))
		if err != nil {
			return err
		}

		ch := make(chan SharedInfo)
		ic.SyncClient.MustPublishSubscribe(ctx,
			tgsync.NewTopic("thread-"+round, SharedInfo{}),
			SharedInfo{ic.GlobalSeq, theThread.Addrs[0].String(), theThread.Key.String()},
			ch)
		// wait until the records from all threads are received
		var wg sync.WaitGroup
		wg.Add(numPeers)
		go func() {
			wg.Wait()
			env.R().RecordPoint("round-"+round+"-elapsed-seconds", time.Since(start).Seconds())
			ic.SyncClient.MustSignalEntry(ctx, doneState)
		}()
		for i := 0; i < numPeers; i++ {
			shared := <-ch
			addr, _ := multiaddr.NewMultiaddr(shared.Addr)
			id, _ := corethread.FromAddr(addr)
			peer := shared.GlobalSeq
			if id != theThread.ID {
				key, _ := corethread.KeyFromString(shared.Key)
				debug("adding thread %v from peer #%d...", addr, peer)
				_, err := client.AddThread(ctx, addr, corenet.WithThreadKey(key))
				if err != nil {
					msg("failed to add thread %v from peer #%d: %v", addr, peer, err)
					return err
				}
				debug("added thread %v from peer #%d, addr %v", id, peer, addr)
			}
			records := make([]corenet.ThreadRecord, 0, numRecords)
			go func() {
				ch, err := client.Subscribe(ctx, corenet.WithSubFilter(id))
				if err != nil {
					msg("Error subscribing thread %v from peer #%d: %v", id, peer, err)
					return
				}
				debug("Subscribed to thread from peer #%d", peer)
				for record := range ch {
					records = append(records, record)
					debug("Records from peer #%d so far: %d", peer, len(records))
					if len(records) >= numRecords {
						msg("Got all %d records from peer #%d", len(records), peer)
						wg.Done()
						return
					}
				}
			}()

		}

		for i := 0; i < numRecords; i++ {
			body, err := ipldcbor.WrapObject(map[string]interface{}{
				"foo": "bar",
				"baz": []byte("howdy"),
			}, multihash.SHA2_256, -1)
			_, err = client.CreateRecord(ctx, theThread.ID, body)
			if err != nil {
				return err
			}
		}
		msg("Peer #%d shoot out %d records", ic.GlobalSeq, numRecords)

		// wait until all instances get correct results
		<-ic.SyncClient.MustBarrier(ctx, doneState, numPeers).C
		return nil
	}

	for _, pair := range netConditions {
		round := pair.simulation
		self := ic.GlobalSeq
		readyState := tgsync.State("ready-" + round)
		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		if self == 1 {
			msg("################### Round %s network ###################", round)
			// peer 1: reconfigure network, then signal others to continue
			ic.NetClient.MustConfigureNetwork(ctx, &network.Config{
				Network:       "default",
				Enable:        true,
				Default:       pair.shape,
				CallbackState: tgsync.State("network-configured-" + round),
				RoutingPolicy: network.AllowAll,
			})
			ic.SyncClient.MustSignalEntry(ctx, readyState)
		} else {
			<-ic.SyncClient.MustBarrier(ctx, readyState, 1).C
		}
		msg("Peer #%d starting round %s", self, round)
		if err := doTest(round); err != nil {
			msg("################### Peer #%d round %s failed: %v ###################", self, round, err)
			return err
		}
	}
	return nil
}

type SharedInfo struct {
	GlobalSeq int64
	Addr      string // multiaddr.Multiaddr
	Key       string // corethread.Key
}

func setup(env *runtime.RunEnv, ic *run.InitContext) (hostAddr string) {
	debug := env.BooleanParam("debug")
	logLevel := logging.LevelError
	if debug {
		logLevel = logging.LevelDebug
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
