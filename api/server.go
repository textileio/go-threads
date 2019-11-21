package api

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	logging "github.com/ipfs/go-log"
	tserv "github.com/textileio/go-textile-core/threadservice"
	pb "github.com/textileio/go-textile-threads/api/pb"
	es "github.com/textileio/go-textile-threads/eventstore"
	"github.com/textileio/go-textile-threads/util"
	logger "github.com/whyrusleeping/go-logging"
	"google.golang.org/grpc"
)

var (
	log = logging.Logger("api")
)

type Server struct {
	rpc     *grpc.Server
	proxy   *http.Server
	service *service

	ctx    context.Context
	cancel context.CancelFunc
}

type Config struct {
	RepoPath  string
	Addr      string // defaults to 0.0.0.0:9090
	ProxyAddr string // defaults to 0.0.0.0:9091
	Debug     bool
}

func NewServer(ctx context.Context, ts tserv.Threadservice, conf Config) (*Server, error) {
	var err error
	if conf.Debug {
		err = util.SetLogLevels(map[string]logger.Level{
			"api": logger.DEBUG,
		})
		if err != nil {
			return nil, err
		}
	}

	manager, err := es.NewManager(
		ts,
		es.WithJsonMode(true),
		es.WithRepoPath(conf.RepoPath),
		es.WithDebug(true))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	s := &Server{
		rpc:     grpc.NewServer(),
		service: &service{manager: manager},
		ctx:     ctx,
		cancel:  cancel,
	}

	if conf.Addr == "" {
		conf.Addr = "0.0.0.0:9090"
	}
	listener, err := net.Listen("tcp", conf.Addr)
	if err != nil {
		return nil, err
	}
	go func() {
		pb.RegisterAPIServer(s.rpc, s.service)
		s.rpc.Serve(listener)
	}()

	webrpc := grpcweb.WrapServer(
		s.rpc,
		grpcweb.WithOriginFunc(func(origin string) bool {
			return true
		}),
		grpcweb.WithWebsockets(true),
		grpcweb.WithWebsocketOriginFunc(func(req *http.Request) bool {
			return true
		}))
	if conf.ProxyAddr == "" {
		conf.ProxyAddr = "0.0.0.0:9091"
	}
	s.proxy = &http.Server{
		Addr: conf.ProxyAddr,
	}
	s.proxy.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if webrpc.IsGrpcWebRequest(r) ||
			webrpc.IsAcceptableGrpcCorsRequest(r) ||
			webrpc.IsGrpcWebSocketRequest(r) {
			webrpc.ServeHTTP(w, r)
		}
	})

	errc := make(chan error)
	go func() {
		errc <- s.proxy.ListenAndServe()
		close(errc)
	}()
	go func() {
		for err := range errc {
			if err != nil {
				if err == http.ErrServerClosed {
					break
				} else {
					log.Errorf("proxy error: %s", err)
				}
			}
		}
		log.Info("proxy was shutdown")
	}()
	log.Infof("proxy listening at %s", s.proxy.Addr)

	return s, nil
}

func (s *Server) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.proxy.Shutdown(ctx); err != nil {
		log.Errorf("error shutting down proxy: %s", err)
	}

	s.rpc.GracefulStop()
	s.service.manager.Close()
	s.cancel()
}
