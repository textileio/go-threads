package gateway

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	mbase "github.com/multiformats/go-multibase"
	"github.com/rs/cors"
	gincors "github.com/rs/cors/wrapper/gin"
	coredb "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/did"
	core "github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
)

var log = logging.Logger("threads/gateway")

const handlerTimeout = time.Minute

func init() {
	gin.SetMode(gin.ReleaseMode)
}

// Gateway provides HTTP-based access to buckets.
type Gateway struct {
	db     *db.Manager
	server *http.Server

	addr       string
	url        string
	subdomains bool
}

// NewGateway returns a new gateway.
func NewGateway(db *db.Manager, addr, url string, subdomains bool) (*Gateway, error) {
	return &Gateway{
		db:         db,
		addr:       addr,
		url:        url,
		subdomains: subdomains,
	}, nil
}

// Start the gateway.
func (g *Gateway) Start() {
	router := gin.Default()

	router.Use(location.Default())
	router.Use(gincors.New(cors.Options{}))

	router.GET("/health", func(c *gin.Context) {
		c.Writer.WriteHeader(http.StatusNoContent)
	})

	router.GET("/thread/:thread/:collection", g.subdomainOptionHandler, g.collectionHandler)
	router.GET("/thread/:thread/:collection/:id", g.subdomainOptionHandler, g.instanceHandler)

	router.NoRoute(render404)

	g.server = &http.Server{
		Addr:    g.addr,
		Handler: router,
	}
	go func() {
		if err := g.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("gateway error: %s", err)
		}
	}()
	log.Infof("gateway listening at %s", g.server.Addr)
}

// Addr returns the gateway's address.
func (g *Gateway) Addr() string {
	return g.server.Addr
}

// Close the gateway.
func (g *Gateway) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := g.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutting down server: %v", err)
	}
	return nil
}

// render404 renders a JSON 404 message.
// @todo: Use this
func render404(c *gin.Context) {
	renderError(c, http.StatusNotFound, nil)
}

// renderError renders a JSON error message.
func renderError(c *gin.Context, code int, err error) {
	if err == nil {
		err = errors.New(http.StatusText(code))
	}
	c.JSON(code, gin.H{"code": code, "message": err.Error()})
}

// collectionHandler handles collection requests.
func (g *Gateway) collectionHandler(c *gin.Context) {
	thread, err := core.Decode(c.Param("thread"))
	if err != nil {
		renderError(c, http.StatusBadRequest, errors.New("invalid thread ID"))
		return
	}
	g.renderCollection(c, thread, c.Param("collection"))
}

// renderCollection renders all instances in a collection.
func (g *Gateway) renderCollection(c *gin.Context, thread core.ID, collection string) {
	token := did.Token(c.Query("token"))

	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()
	d, err := g.db.GetDB(ctx, thread, db.WithManagedToken(token))
	if err != nil {
		render404(c)
		return
	}
	col := d.GetCollection(collection, db.WithToken(token))
	if c == nil {
		render404(c)
		return
	}
	res, err := col.Find(&db.Query{}, db.WithTxnToken(token))
	if err != nil {
		render404(c)
		return
	}
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, r := range res {
		if _, err := buf.Write(r); err != nil {
			renderError(c, http.StatusInternalServerError, fmt.Errorf("writing response: %v", err))
			return
		}
		if i != len(res)-1 {
			buf.WriteByte(',')
		}
	}
	buf.WriteByte(']')
	c.Render(200, render.Data{Data: buf.Bytes(), ContentType: "application/json"})
}

// instanceHandler handles collection instance requests.
func (g *Gateway) instanceHandler(c *gin.Context) {
	thread, err := core.Decode(c.Param("thread"))
	if err != nil {
		renderError(c, http.StatusBadRequest, errors.New("invalid thread ID"))
		return
	}
	g.renderInstance(c, thread, c.Param("collection"), c.Param("id"))
}

// renderInstance renders an instance in a collection.
func (g *Gateway) renderInstance(c *gin.Context, thread core.ID, collection, id string) {
	token := did.Token(c.Query("token"))

	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()
	d, err := g.db.GetDB(ctx, thread, db.WithManagedToken(token))
	if err != nil {
		render404(c)
		return
	}
	col := d.GetCollection(collection, db.WithToken(token))
	if c == nil {
		render404(c)
		return
	}
	res, err := col.FindByID(coredb.InstanceID(id), db.WithTxnToken(token))
	if err != nil {
		render404(c)
		return
	}
	c.Render(200, render.Data{Data: res, ContentType: "application/json"})
}

// subdomainOptionHandler redirects valid namespaces to subdomains if the option is enabled.
func (g *Gateway) subdomainOptionHandler(c *gin.Context) {
	if !g.subdomains {
		return
	}
	loc, ok := g.toSubdomainURL(c.Request)
	if !ok {
		render404(c)
		return
	}

	// See security note:
	// https://github.com/ipfs/go-ipfs/blob/dbfa7bf2b216bad9bec1ff66b1f3814f4faac31e/core/corehttp/hostname.go#L105
	c.Request.Header.Set("Clear-Site-Data", "\"cookies\", \"storage\"")

	c.Redirect(http.StatusPermanentRedirect, loc)
}

// subdomainHandler handles requests by parsing the request subdomain.
func (g *Gateway) subdomainHandler(c *gin.Context) {
	c.Status(200)

	parts := strings.Split(c.Request.Host, ".")
	key := parts[0]

	if len(parts) < 3 {
		render404(c)
		return
	}
	ns := parts[1]
	if !isSubdomainNamespace(ns) {
		render404(c)
		return
	}
	switch ns {
	case "thread":
		thread, err := core.Decode(key)
		if err != nil {
			renderError(c, http.StatusBadRequest, errors.New("invalid thread ID"))
			return
		}
		parts := strings.SplitN(strings.TrimSuffix(c.Request.URL.Path, "/"), "/", 4)
		switch len(parts) {
		case 1:
			// @todo: Render something at the thread root
			render404(c)
		case 2:
			if parts[1] != "" {
				g.renderCollection(c, thread, parts[1])
			} else {
				render404(c)
			}
		case 3:
			g.renderInstance(c, thread, parts[1], parts[2])
		default:
			render404(c)
		}
	default:
		render404(c)
	}
}

// Modified from:
// https://github.com/ipfs/go-ipfs/blob/dbfa7bf2b216bad9bec1ff66b1f3814f4faac31e/core/corehttp/hostname.go#L251
func isSubdomainNamespace(ns string) bool {
	switch ns {
	case "thread":
		return true
	default:
		return false
	}
}

// Converts a hostname/path to a subdomain-based URL, if applicable.
// Modified from:
// https://github.com/ipfs/go-ipfs/blob/dbfa7bf2b216bad9bec1ff66b1f3814f4faac31e/core/corehttp/hostname.go#L270
func (g *Gateway) toSubdomainURL(r *http.Request) (redirURL string, ok bool) {
	var ns, rootID, rest string

	query := r.URL.RawQuery
	parts := strings.SplitN(r.URL.Path, "/", 4)
	safeRedirectURL := func(in string) (out string, ok bool) {
		safeURI, err := url.ParseRequestURI(in)
		if err != nil {
			return "", false
		}
		return safeURI.String(), true
	}

	switch len(parts) {
	case 4:
		rest = parts[3]
		fallthrough
	case 3:
		ns = parts[1]
		rootID = parts[2]
	default:
		return "", false
	}

	if !isSubdomainNamespace(ns) {
		return "", false
	}

	// add prefix if query is present
	if query != "" {
		query = "?" + query
	}

	// If rootID is a CID, ensure it uses DNS-friendly text representation
	if rootCid, err := cid.Decode(rootID); err == nil {
		multicodec := rootCid.Type()

		// if object turns out to be a valid CID,
		// ensure text representation used in subdomain is CIDv1 in Base32
		// https://github.com/ipfs/in-web-browsers/issues/89
		rootID, err = cid.NewCidV1(multicodec, rootCid.Hash()).StringOfBase(mbase.Base32)
		if err != nil {
			// should not error, but if it does, its clealy not possible to
			// produce a subdomain URL
			return "", false
		}
	}

	urlparts := strings.Split(g.url, "://")
	if len(urlparts) < 2 {
		return "", false
	}
	scheme := urlparts[0]
	host := urlparts[1]
	return safeRedirectURL(fmt.Sprintf("%s://%s.%s.%s/%s%s", scheme, rootID, ns, host, rest, query))
}
