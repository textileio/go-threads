package ws

import (
	"context"
	"net/http"
	"time"

	logging "github.com/ipfs/go-log"
	tserv "github.com/textileio/go-textile-core/threadservice"
)

var log = logging.Logger("ws")

// Server wraps a connection hub and http server.
type Server struct {
	hub *Hub
	s   *http.Server
}

// NewServer returns a web socket server.
func NewServer(addr string) *Server {
	s := &Server{
		hub: newHub(),
	}
	go s.hub.run()

	s.s = &http.Server{
		Addr: addr,
	}
	s.s.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveWs(s.hub, w, r)
	})

	errc := make(chan error)
	go func() {
		errc <- s.s.ListenAndServe()
		close(errc)
	}()
	go func() {
		for {
			select {
			case err, ok := <-errc:
				if err != nil && err != http.ErrServerClosed {
					log.Errorf("ws server error: %s", err)
				}
				if !ok {
					log.Info("ws server was shutdown")
					return
				}
			}
		}
	}()
	log.Infof("ws server listening at %s", s.s.Addr)

	return s
}

// Send a message to the hub.
func (s *Server) Send(r tserv.Record) {
	s.hub.broadcast <- r
}

// Close the server.
func (s *Server) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.s.Shutdown(ctx); err != nil {
		log.Errorf("error shutting down ws server: %s", err)
	}
}
