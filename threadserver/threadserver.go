package threadserver

import (
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/prometheus/common/log"
	cors "github.com/rs/cors/wrapper/gin"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
	"github.com/textileio/go-textile-threads/threadserver/static/css"
	"github.com/textileio/go-textile-threads/threadserver/templates"
)

const (
	defaultNodeLimit = 1
)

type threadserver struct {
	service tserv.Threadservice
	server  *http.Server
}

func NewThreadserver(addr string, service tserv.Threadservice) *threadserver {
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()
	server := &threadserver{
		server: &http.Server{
			Addr:    addr,
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

	router.GET("/threads/:thread/logs/:log", server.nodeHandler)

	router.NoRoute(func(g *gin.Context) {
		server.render404(g)
	})

	errc := make(chan error)
	go func() {
		errc <- server.server.ListenAndServe()
		close(errc)
	}()
	go func() {
		for {
			select {
			case err, ok := <-errc:
				if err != nil && err != http.ErrServerClosed {
					//log.Errorf("threadserver error: %s", err)
				}
				if !ok {
					//log.Info("threadserver was shutdown")
					return
				}
			}
		}
	}()
	//log.Infof("threadserver listening at %s", server.Addr)

	return server
}

func (s *threadserver) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.server.Shutdown(ctx); err != nil {
		log.Errorf("error shutting down gateway: %s", err)
		return err
	}
	return nil
}

func (s *threadserver) Addr() string {
	return s.server.Addr
}

func (s *threadserver) nodeHandler(g *gin.Context) {
	tid, err := thread.Decode(g.Param("thread"))
	if err != nil {
		g.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": fmt.Errorf("invalid thread id: %s", err.Error()),
		})
		return
	}

	id, err := peer.IDB58Decode(g.Param("log"))
	if err != nil {
		g.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": fmt.Errorf("invalid log id: %s", err.Error()),
		})
		return
	}

	var limit int
	limitq, found := g.GetQuery("limit")
	if found {
		limit, err = strconv.Atoi(limitq)
	}
	if !found || err != nil {
		limit = defaultNodeLimit
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	events, err := s.service.Pull(ctx, cid.Undef, limit, tid, id)
	if err != nil {
		g.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	g.JSON(http.StatusOK, events)
}

func (s *threadserver) render404(g *gin.Context) {
	g.HTML(http.StatusNotFound, "404", nil)
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
