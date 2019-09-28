package threadserver

import (
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	cors "github.com/rs/cors/wrapper/gin"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
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

	log := s.service().LogInfo(tid, id)
	if log.PubKey == nil {
		g.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("log not found"),
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
	events, err := s.service().Pull(ctx, cid.Undef, limit, log)
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

	// validate node w/ sig and sender key in header

	// do we have this log?
	//   if yes, update it
	//   if no, is this an invite? (decode with follow key in header)
	//     if yes and read key is present, create own log
	//     if yes and no read key is present, add it, don't create own log
	//     if no, ignore it

	g.Status(http.StatusCreated)

	//var logs []thread.LogInfo
	//err = g.BindJSON(&logs)
	//if err != nil {
	//	g.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
	//		"error": err.Error(),
	//	})
	//	return
	//}
	//
	//for _, log := range logs {
	//	err = s.service().AddLog(tid, log)
	//	if err != nil {
	//		g.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
	//			"error": err.Error(),
	//		})
	//		return
	//	}
	//}
	//
	//body, err := cbornode.WrapObject(map[string]interface{}{
	//	"_type": "JOIN",
	//}, mh.SHA2_256, -1)
	//if err != nil {
	//	g.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
	//		"error": err.Error(),
	//	})
	//	return
	//}
	//
	//ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	//defer cancel()
	//id, _, err := s.service().Put(ctx, body, tserv.PutOpt.Thread(tid))
	//if err != nil {
	//	g.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
	//		"error": err.Error(),
	//	})
	//	return
	//}
	//
	//events, err := s.service().Pull(ctx, cid.Undef, 1, s.service().LogInfo(tid, id))
	//if err != nil {
	//	g.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
	//		"error": err.Error(),
	//	})
	//	return
	//}
	//
	//g.JSON(http.StatusOK, gin.H{
	//	"event": events[0],
	//})
}

func (s *Threadserver) render404(g *gin.Context) {
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
