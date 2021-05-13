package main

import (
	"context"
	"net"
	"net/http"
	_ "net/http/pprof"
	"sync"

	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	tgsync "github.com/testground/sdk-go/sync"
	"google.golang.org/grpc"

	ipldcbor "github.com/ipfs/go-ipld-cbor"
	corenet "github.com/textileio/go-threads/core/net"
	corethread "github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/net/api"
	"github.com/textileio/go-threads/net/api/client"
	"github.com/textileio/go-threads/util"
)

func main() {
	run.InvokeMap(map[string]interface{}{
		"sync-threads": SyncThreads,
	})
}

func SyncThreads(runenv *runtime.RunEnv) (err error) {
	var (
		numRecords = runenv.IntParam("records")
		peers      = runenv.TestInstanceCount
	)

	ri := startTestground(runenv)
	defer ri.Stop()

	// starts the API server and client
	_, addr, shutdown, err := api.CreateTestService(runenv.BooleanParam("debug"))
	if err != nil {
		return err
	}
	defer shutdown()
	target, err := util.TCPAddrFromMultiAddr(addr)
	if err != nil {
		return err
	}
	client, err := client.NewClient(target, grpc.WithInsecure(), grpc.WithPerRPCCredentials(corethread.Credentials{}))
	if err != nil {
		return err
	}
	defer client.Close()

	ctx := context.Background()
	theThread, err := client.CreateThread(ctx, corethread.NewIDV1(corethread.Raw, 32))
	if err != nil {
		return err
	}

	ch := make(chan Transferred)
	ri.sync.MustPublishSubscribe(ctx,
		tgsync.NewTopic("thread", Transferred{}),
		Transferred{theThread.Addrs[0].String(), theThread.Key.String()},
		ch)
	// wait until the records from all threads are received
	var wg sync.WaitGroup
	wg.Add(peers)
	go func() {
		wg.Wait()
		ri.Ready()
	}()
	for i := 0; i < peers; i++ {
		shared := <-ch
		addr, _ := multiaddr.NewMultiaddr(shared.Addr)
		id, _ := corethread.FromAddr(addr)
		if id != theThread.ID {
			key, _ := corethread.KeyFromString(shared.Key)
			_, err := client.AddThread(ctx, addr, corenet.WithThreadKey(key))
			if err != nil {
				ri.Msg("failed to add thread %v: %v", addr, err)
				return err
			}
			ri.Msg("added thread %v from %v", id, addr)
		}
		records := make([]corenet.ThreadRecord, 0)
		go func() {
			ch, err := client.Subscribe(ctx, corenet.WithSubFilter(id))
			if err != nil {
				ri.Msg("Error subscribing thread %v: %v", id, err)
				return
			}
			ri.Msg("Subscribed to thread %v", id)
			for record := range ch {
				records = append(records, record)
				ri.Msg("Records from thread %v so far: %d", record.ThreadID(), len(records))
				if len(records) >= numRecords {
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

	// wait until all instances got correct results
	ri.Wait()
	return nil
}

type Transferred struct {
	Addr string // multiaddr.Multiaddr
	Key  string // corethread.Key
}

type runInfo struct {
	runenv  *runtime.RunEnv
	sync    tgsync.Client
	network *network.Client
}

var (
	readyState = tgsync.State("ready")
)

func (ri *runInfo) Ready() {
	seq := ri.sync.MustSignalEntry(context.Background(), readyState)
	ri.Msg("Signalled that peer #%d is ready", seq)
}

func (ri *runInfo) Wait() {
	howMany := ri.runenv.TestInstanceCount
	ri.Msg("Waiting for %v peers to be ready", howMany)
	<-ri.sync.MustBarrier(context.Background(), readyState, howMany).C
	ri.Msg("%v peers ready", howMany)
}

func (ri *runInfo) Msg(msg string, args ...interface{}) {
	ri.runenv.RecordMessage(msg, args...)
}

func (ri *runInfo) Stop() {
	ri.sync.Close()
}

func startTestground(runenv *runtime.RunEnv) *runInfo {
	if runenv.BooleanParam("pprof") {
		go func() {
			l, _ := net.Listen("tcp", ":0")
			runenv.RecordMessage("starting pprof at %s/debug/pprof", l.Addr().String())
			_ = http.Serve(l, nil)
		}()
	}

	ctx := context.Background()
	cli := tgsync.MustBoundClient(ctx, runenv)
	net := network.NewClient(cli, runenv)
	net.MustWaitNetworkInitialized(ctx)
	return &runInfo{runenv, cli, net}
}
