package threadserver

//
//import (
//	"context"
//	"encoding/base64"
//	"fmt"
//	"html/template"
//	"io/ioutil"
//	"net"
//	"net/http"
//	"strconv"
//	"time"
//
//	"github.com/textileio/go-textile-core/crypto"
//
//	"github.com/gin-contrib/location"
//	"github.com/gin-gonic/gin"
//	"github.com/gin-gonic/gin/render"
//	ic "github.com/libp2p/go-libp2p-core/crypto"
//	"github.com/libp2p/go-libp2p-core/peer"
//	cors "github.com/rs/cors/wrapper/gin"
//	"github.com/textileio/go-textile-core/crypto/asymmetric"
//	"github.com/textileio/go-textile-core/thread"
//	tserv "github.com/textileio/go-textile-core/threadservice"
//	"github.com/textileio/go-textile-threads/cbor"
//	"github.com/textileio/go-textile-threads/threadserver/static/css"
//	"github.com/textileio/go-textile-threads/threadserver/templates"
//	"github.com/textileio/go-textile-threads/util"
//)
//
//const (
//	defaultPullLimit = 1
//)
//
//type Threadserver struct {
//	service func() tserv.Threadservice
//	server  *http.Server
//}
//
//func NewThreadserver(service func() tserv.Threadservice) *Threadserver {
//	gin.SetMode(gin.ReleaseMode)
//
//	router := gin.Default()
//	server := &Threadserver{
//		server: &http.Server{
//			Handler: router,
//		},
//		service: service,
//	}
//
//	router.Use(location.Default())
//
//	router.Use(cors.New(cors.Options{
//		AllowedOrigins: []string{"*"},
//		AllowedMethods: []string{"GET"},
//		AllowedHeaders: []string{"X-Requested-With", "Range"},
//	}))
//
//	router.SetHTMLTemplate(parseTemplates())
//
//	router.GET("/favicon.ico", func(g *gin.Context) {
//		img, err := base64.StdEncoding.DecodeString(favicon)
//		if err != nil {
//			g.Writer.WriteHeader(http.StatusNotFound)
//			return
//		}
//		g.Header("Cache-Control", "public, max-age=172800")
//		g.Render(http.StatusOK, render.Data{Data: img})
//	})
//	router.GET("/static/css/style.css", func(g *gin.Context) {
//		g.Header("Content-Type", "text/css; charset=utf-8")
//		g.Header("Cache-Control", "public, max-age=172800")
//		g.String(http.StatusOK, css.Style)
//	})
//
//	v0 := router.Group("/threads/v0")
//	{
//		v0.GET("/:thread/:id", server.pullHandler)
//		v0.POST("/:thread/:id", server.eventHandler)
//	}
//
//	router.NoRoute(func(g *gin.Context) {
//		server.render404(g)
//	})
//
//	return server
//}
//
//func (s *Threadserver) Open(listener net.Listener) {
//	errc := make(chan error)
//	go func() {
//		errc <- s.server.Serve(listener)
//		close(errc)
//	}()
//	go func() {
//		for {
//			select {
//			case err, ok := <-errc:
//				if err != nil && err != http.ErrServerClosed {
//					//log.Errorf("Threadserver error: %s", err)
//				}
//				if !ok {
//					//log.Info("Threadserver was shutdown")
//					return
//				}
//			}
//		}
//	}()
//	//log.Infof("Threadserver listening at %s", server.Addr)
//}
//
//func (s *Threadserver) Close() error {
//	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
//	defer cancel()
//	return s.server.Shutdown(ctx)
//}
//
//func (s *Threadserver) pullHandler(g *gin.Context) {
//	tid, err := thread.Decode(g.Param("thread"))
//	if err != nil {
//		s.error(g, http.StatusBadRequest, "invalid thread id", err)
//		return
//	}
//
//	id, err := peer.IDB58Decode(g.Param("id"))
//	if err != nil {
//		s.error(g, http.StatusBadRequest, "invalid log id", err)
//		return
//	}
//
//	var limit int
//	limitq, found := g.GetQuery("limit")
//	if found {
//		limit, err = strconv.Atoi(limitq)
//	}
//	if !found || err != nil {
//		limit = defaultPullLimit
//	}
//
//	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
//	defer cancel()
//	events, err := s.service().Pull(ctx, tid, id, tserv.PullOpt.Limit(limit))
//	if err != nil {
//		s.error(g, http.StatusInternalServerError, "oops", err)
//		return
//	}
//
//	g.JSON(http.StatusOK, events)
//}
//
//func (s *Threadserver) eventHandler(g *gin.Context) {
//	tid, err := thread.Decode(g.Param("thread"))
//	if err != nil {
//		s.error(g, http.StatusBadRequest, "invalid thread id", err)
//		return
//	}
//	id, err := peer.IDB58Decode(g.Param("id"))
//	if err != nil {
//		s.error(g, http.StatusBadRequest, "invalid log id", err)
//		return
//	}
//	pkb, err := base64.StdEncoding.DecodeString(g.Request.Header.Get("X-Identity"))
//	if err != nil {
//		s.error(g, http.StatusBadRequest, "invalid identity", err)
//		return
//	}
//	pk, err := ic.UnmarshalPublicKey(pkb)
//	if err != nil {
//		s.error(g, http.StatusBadRequest, "invalid identity", err)
//		return
//	}
//	sig, err := base64.StdEncoding.DecodeString(g.Request.Header.Get("X-Signature"))
//	if err != nil {
//		s.error(g, http.StatusBadRequest, "invalid signature", err)
//		return
//	}
//	fk, err := base64.StdEncoding.DecodeString(g.Request.Header.Get("X-FollowKey"))
//	if err != nil {
//		s.error(g, http.StatusBadRequest, "invalid key", err)
//		return
//	}
//
//	body, err := ioutil.ReadAll(g.Request.Body)
//	if err != nil {
//		s.error(g, http.StatusBadRequest, "invalid event", err)
//		return
//	}
//	defer g.Request.Body.Close()
//
//	// Verify sender
//	ok, err := pk.Verify(body, sig)
//	if !ok || err != nil {
//		s.error(g, http.StatusBadRequest, "invalid event", fmt.Errorf("bad signature"))
//		return
//	}
//
//	followKey, err := crypto.ParseDecryptionKey(fk)
//	if err != nil {
//		s.error(g, http.StatusBadRequest, "invalid event", err)
//		return
//	}
//	node, err := cbor.Unmarshal(body, followKey)
//	if err != nil {
//		s.error(g, http.StatusBadRequest, "invalid event", err)
//		return
//	}
//
//	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
//	defer cancel()
//
//	var res thread.Node
//	var nlog *thread.LogInfo
//	lpk := s.service().PubKey(tid, id)
//	if lpk == nil {
//		// This is a new log
//		event, err := cbor.EventFromRecord(ctx, s.service().DAGService(), node)
//		if err != nil {
//			s.error(g, http.StatusBadRequest, "invalid event", err)
//			return
//		}
//
//		res, nlog, err = s.handleInvite(ctx, tid, id, event)
//		if err != nil {
//			s.error(g, http.StatusBadRequest, "invalid event", err)
//			return
//		}
//		lpk = s.service().PubKey(tid, id) // @todo: This should happen before handling the invite
//		if lpk == nil || !id.MatchesPublicKey(lpk) {
//			s.error(g, http.StatusBadRequest, "invalid event", fmt.Errorf("bad pubkey"))
//			return
//		}
//	}
//
//	// Verify node
//	err = node.Verify(lpk)
//	if err != nil {
//		s.error(g, http.StatusBadRequest, "invalid event", err)
//		return
//	}
//
//	err = s.service().Put(ctx, node, tserv.PutOpt.Thread(tid), tserv.PutOpt.Log(id))
//	if err != nil {
//		s.error(g, http.StatusBadRequest, "invalid event", err)
//		return
//	}
//
//	if res != nil {
//		payload, err := cbor.Marshal(ctx, s.service().DAGService(), res)
//		if err != nil {
//			s.error(g, http.StatusInternalServerError, "oops", err)
//			return
//		}
//		g.Writer.Header().Set("X-FollowKey", base64.StdEncoding.EncodeToString(nlog.FollowKey))
//		g.Writer.Header().Set("X-ReadKey", base64.StdEncoding.EncodeToString(nlog.ReadKey))
//
//		g.Data(http.StatusCreated, "application/cbor", payload)
//	} else {
//		g.Status(http.StatusNoContent)
//	}
//}
//
//func (s *Threadserver) handleInvite(ctx context.Context, t thread.ID, l peer.ID, event thread.Event) (thread.Node, *thread.LogInfo, error) {
//	sk := s.service().Host().Peerstore().PrivKey(s.service().Host().ID())
//	if sk == nil {
//		return nil, nil, fmt.Errorf("private key not found")
//	}
//	key, err := asymmetric.NewDecryptionKey(sk)
//	if err != nil {
//		return nil, nil, err
//	}
//	body, err := event.GetBody(ctx, s.service().DAGService(), key)
//	if err != nil {
//		return nil, nil, err
//	}
//	logs, reader, err := cbor.InviteFromNode(body)
//	if err != nil {
//		return nil, nil, err
//	}
//
//	// Add own log first because Add will attempt to notify the other logs,
//	// which is not needed here.
//	var node thread.Node
//	var nlog thread.LogInfo
//	if reader {
//		nlog, err = util.CreateLog(s.service().Host().ID())
//		if err != nil {
//			return nil, nil, err
//		}
//		err = s.service().AddLog(t, nlog)
//		if err != nil {
//			return nil, nil, err
//		}
//
//		// Create an invite for the response
//		invite, err := cbor.NewInvite([]thread.LogInfo{nlog}, true)
//		if err != nil {
//			return nil, nil, err
//		}
//		_, node, err = s.service().Add(ctx, invite, tserv.AddOpt.Thread(t))
//		if err != nil {
//			return nil, nil, err
//		}
//	}
//
//	// Add additional logs
//	for _, log := range logs {
//		err = s.service().AddLog(t, log)
//		if err != nil {
//			return nil, nil, err
//		}
//	}
//
//	return node, &nlog, nil
//}
//
//func (s *Threadserver) error(g *gin.Context, status int, prefix string, err error) {
//	msg := err.Error()
//	if prefix != "" {
//		msg = prefix + ": " + msg
//	}
//	g.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": msg})
//}
//
//func (s *Threadserver) render404(g *gin.Context) {
//	g.HTML(http.StatusNotFound, "404", nil)
//}
//
//func parseTemplates() *template.Template {
//	temp, err := template.New("index").Parse(templates.Index)
//	if err != nil {
//		panic(err)
//	}
//	temp, err = temp.New("404").Parse(templates.NotFound)
//	if err != nil {
//		panic(err)
//	}
//	return temp
//}
