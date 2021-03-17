package gateway

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	coredb "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/did"
	core "github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
)

// collectionHandler handles collection requests.
func (g *Gateway) collectionHandler(c *gin.Context) {
	thread, err := core.Decode(c.Param("thread"))
	if err != nil {
		renderError(c, http.StatusBadRequest, fmt.Errorf("invalid thread ID"))
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
	c.JSON(http.StatusOK, res)
}

// instanceHandler handles collection instance requests.
func (g *Gateway) instanceHandler(c *gin.Context) {
	thread, err := core.Decode(c.Param("thread"))
	if err != nil {
		renderError(c, http.StatusBadRequest, fmt.Errorf("invalid thread ID"))
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
	c.JSON(http.StatusOK, res)
}
