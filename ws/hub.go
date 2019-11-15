// Inspired by https://github.com/gorilla/websocket/tree/master/examples/chat with
// adaptations for multiple rooms ("threads" in Textile parlance) and authentication.
package ws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	format "github.com/ipfs/go-ipld-format"
	"github.com/mr-tron/base58"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
	"github.com/textileio/go-textile-threads/cbor"
)

var encodeRecordTimeout = time.Second * 5

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	ctx context.Context

	// service provides thread access.
	service tserv.Threadservice

	// clients currently registered.
	clients map[*Client]struct{}

	// broadcast records to clients.
	broadcast chan tserv.Record

	// register requests from the clients.
	register chan *Client

	// unregister requests from clients.
	unregister chan *Client
}

// NewHub creates a new client hub.
func newHub(ctx context.Context, ts tserv.Threadservice) *Hub {
	return &Hub{
		ctx:        ctx,
		service:    ts,
		broadcast:  make(chan tserv.Record),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]struct{}),
	}
}

// Run the hub.
func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = struct{}{}
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case rec := <-h.broadcast:
			rid := rec.Value().Cid().String()

			ctx, cancel := context.WithTimeout(h.ctx, encodeRecordTimeout)
			jrec, err := recordToJSON(ctx, h.service, rec.Value())
			if err != nil {
				log.Errorf("error converting record %s to JSON: %v", rid, err)
				cancel()
				break
			}
			cancel()

			jrec.LogID = rec.LogID().String()
			jrec.ThreadID = rec.ThreadID().String()

			msg, err := json.Marshal(jrec)
			if err != nil {
				log.Errorf("error marshaling record %s: %v", rid, err)
				break
			}

			for client := range h.clients {
				if _, ok := client.threads[rec.ThreadID()]; !ok {
					continue
				}
				select {
				case client.send <- msg:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

// jsonThreadInfo is thread info in JSON.
type jsonThreadInfo struct {
	ID        string   `json:"id"`
	Logs      []string `json:"logs"`
	FollowKey string   `json:"follow_key"`
	ReadKey   string   `json:"read_key,omitempty"`
}

// threadInfoToJSON returns a JSON version of thread info for transport.
func threadInfoToJSON(info thread.Info) *jsonThreadInfo {
	logs := make([]string, 0, len(info.Logs))
	for _, lg := range info.Logs {
		logs = append(logs, lg.String())
	}
	return &jsonThreadInfo{
		ID:        info.ID.String(),
		Logs:      logs,
		FollowKey: base58.Encode(info.FollowKey.Bytes()),
		ReadKey:   base58.Encode(info.FollowKey.Bytes()),
	}
}

// jsonRecord is a thread record containing link data in JSON.
type jsonRecord struct {
	ID       string `json:"id"`
	LogID    string `json:"log_id,omitempty"`
	ThreadID string `json:"thread_id"`

	RecordNode string `json:"record_node,omitempty"`
	EventNode  string `json:"event_node,omitempty"`
	HeaderNode string `json:"header_node,omitempty"`
	BodyNode   string `json:"body_node,omitempty"`
}

// recordToJSON returns a JSON version of a record for transport.
// Nodes are sent encrypted.
func recordToJSON(ctx context.Context, dag format.DAGService, rec thread.Record) (*jsonRecord, error) {
	block, err := rec.GetBlock(ctx, dag)
	if err != nil {
		return nil, err
	}
	event, err := cbor.EventFromNode(block)
	if err != nil {
		return nil, err
	}
	header, err := event.GetHeader(ctx, dag, nil)
	if err != nil {
		return nil, err
	}
	body, err := event.GetBody(ctx, dag, nil)
	if err != nil {
		return nil, err
	}

	return &jsonRecord{
		ID:         rec.Cid().String(),
		RecordNode: encodeNode(rec),
		EventNode:  encodeNode(block),
		HeaderNode: encodeNode(header),
		BodyNode:   encodeNode(body),
	}, nil
}

// encodeNode returns a base64-encoded version of n.
func encodeNode(n format.Node) string {
	return base64.StdEncoding.EncodeToString(n.RawData())
}
