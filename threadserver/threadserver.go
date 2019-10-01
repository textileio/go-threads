package threadserver

import (
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	cbornode "github.com/ipfs/go-ipld-cbor"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	mh "github.com/multiformats/go-multihash"
	cors "github.com/rs/cors/wrapper/gin"
	"github.com/textileio/go-textile-core/crypto"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
	"github.com/textileio/go-textile-threads/cbor"
	"github.com/textileio/go-textile-threads/threadserver/static/css"
	"github.com/textileio/go-textile-threads/threadserver/templates"
)

const (
	defaultPullLimit = 1
)

type Threadserver struct {
	service func() tserv.Threadservice
	server  *http.Server
}

func NewThreadserver(service func() tserv.Threadservice) *Threadserver {
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()
	server := &Threadserver{
		server: &http.Server{
			Handler: router,
		},
		service: service,
	}

	router.Use(location.Default())

	router.Use(cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET"},
		AllowedHeaders: []string{"X-Requested-With", "Range"},
	}))

	router.SetHTMLTemplate(parseTemplates())

	router.GET("/favicon.ico", func(g *gin.Context) {
		img, err := base64.StdEncoding.DecodeString(favicon)
		if err != nil {
			g.Writer.WriteHeader(http.StatusNotFound)
			return
		}
		g.Header("Cache-Control", "public, max-age=172800")
		g.Render(http.StatusOK, render.Data{Data: img})
	})
	router.GET("/static/css/style.css", func(g *gin.Context) {
		g.Header("Content-Type", "text/css; charset=utf-8")
		g.Header("Cache-Control", "public, max-age=172800")
		g.String(http.StatusOK, css.Style)
	})

	v0 := router.Group("/threads/v0")
	{
		v0.GET("/:thread/:id", server.pullHandler)
		v0.POST("/:thread/:id", server.eventHandler)
	}

	router.NoRoute(func(g *gin.Context) {
		server.render404(g)
	})

	return server
}

func (s *Threadserver) Open(listener net.Listener) {
	errc := make(chan error)
	go func() {
		errc <- s.server.Serve(listener)
		close(errc)
	}()
	go func() {
		for {
			select {
			case err, ok := <-errc:
				if err != nil && err != http.ErrServerClosed {
					//log.Errorf("Threadserver error: %s", err)
				}
				if !ok {
					//log.Info("Threadserver was shutdown")
					return
				}
			}
		}
	}()
	//log.Infof("Threadserver listening at %s", server.Addr)
}

func (s *Threadserver) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func (s *Threadserver) pullHandler(g *gin.Context) {
	tid, err := thread.Decode(g.Param("thread"))
	if err != nil {
		g.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid thread id: %s", err.Error()),
		})
		return
	}

	id, err := peer.IDB58Decode(g.Param("id"))
	if err != nil {
		g.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid log id: %s", err.Error()),
		})
		return
	}

	var limit int
	limitq, found := g.GetQuery("limit")
	if found {
		limit, err = strconv.Atoi(limitq)
	}
	if !found || err != nil {
		limit = defaultPullLimit
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	events, err := s.service().Pull(ctx, tid, id, tserv.PullOpt.Limit(limit))
	if err != nil {
		g.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	g.JSON(http.StatusOK, events)
}

func (s *Threadserver) eventHandler(g *gin.Context) {
	tid, err := thread.Decode(g.Param("thread"))
	if err != nil {
		g.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid thread id: %s", err.Error()),
		})
		return
	}
	id, err := peer.IDB58Decode(g.Param("id"))
	if err != nil {
		g.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid log id: %s", err.Error()),
		})
		return
	}
	pkb, err := base64.StdEncoding.DecodeString(g.Request.Header.Get("Identity"))
	if err != nil {
		g.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "invalid identity",
		})
		return
	}
	pk, err := ic.UnmarshalPublicKey(pkb)
	if err != nil {
		g.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "invalid identity",
		})
		return
	}
	sig, err := base64.StdEncoding.DecodeString(g.Request.Header.Get("Signature"))
	if err != nil {
		g.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "invalid signature",
		})
		return
	}
	fk, err := base64.StdEncoding.DecodeString(g.Request.Header.Get("Authorization"))
	if err != nil {
		g.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "invalid authorization",
		})
		return
	}

	body, err := ioutil.ReadAll(g.Request.Body)
	if err != nil {
		g.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "invalid event",
		})
		return
	}
	defer g.Request.Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	node, err := s.decodeBody(ctx, body, fk)
	if err != nil {
		g.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "invalid event",
		})
		return
	}

	// @todo: validate node w/ sig and sender pk

	log := s.service().LogInfo(tid, id)
	if log.PubKey == nil {
		// This is a new log
		// @todo: parse as invite (decyrpt w/ own sk)
		// @todo: add log
		// @todo: if read key present, create own log
		// @todo: if read key present, respond w/ new invite
	}

	// @todo: put node (add local, update head)

	g.Status(http.StatusCreated)
}

func (s *Threadserver) render404(g *gin.Context) {
	g.HTML(http.StatusNotFound, "404", nil)
}

func (s *Threadserver) decodeBody(ctx context.Context, body []byte, fk []byte) (thread.Node, error) {
	node, err := cbornode.Decode(body, mh.SHA2_256, -1)
	if err != nil {
		return nil, err
	}
	followKey, err := crypto.ParseDecryptionKey(fk)
	if err != nil {
		return nil, err
	}
	return cbor.DecodeNode(ctx, s.service().DAGService(), node, followKey)
}

func parseTemplates() *template.Template {
	temp, err := template.New("index").Parse(templates.Index)
	if err != nil {
		panic(err)
	}
	temp, err = temp.New("404").Parse(templates.NotFound)
	if err != nil {
		panic(err)
	}
	return temp
}
