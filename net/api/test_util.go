package api

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"time"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/phayes/freeport"
	"github.com/textileio/go-threads/common"
	pb "github.com/textileio/go-threads/net/api/pb"
	"github.com/textileio/go-threads/util"
	"google.golang.org/grpc"
)

// CreateTestService creates a test network API gRPC service for test purpose
func CreateTestService(debug bool) (hostAddr ma.Multiaddr, gRPCAddr ma.Multiaddr, stop func(), err error) {
	time.Sleep(time.Second * time.Duration(rand.Intn(5)))
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return
	}
	hostAddr = util.FreeLocalAddr()
	n, err := common.DefaultNetwork(
		common.WithNetBadgerPersistence(dir),
		common.WithNetHostAddr(hostAddr),
		common.WithNetPubSub(true),
		common.WithNetDebug(debug),
	)
	if err != nil {
		return
	}
	service, err := NewService(n, Config{
		Debug: debug,
	})
	if err != nil {
		return
	}
	port, err := freeport.GetFreePort()
	if err != nil {
		return
	}
	addr := util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port))
	target, err := util.TCPAddrFromMultiAddr(addr)
	if err != nil {
		return
	}
	server := grpc.NewServer()
	listener, err := net.Listen("tcp", target)
	if err != nil {
		return
	}
	go func() {
		pb.RegisterAPIServer(server, service)
		if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Fatalf("serve error: %v", err)
		}
	}()

	return hostAddr, addr, func() {
		server.GracefulStop()
		if err := n.Close(); err != nil {
			return
		}
		_ = os.RemoveAll(dir)
	}, nil
}
