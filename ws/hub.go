// Inspired by https://github.com/gorilla/websocket/tree/master/examples/chat with
// adaptations for multiple rooms ("threads" in Textile parlance) and authentication.
package ws

import (
	"encoding/base64"

	tserv "github.com/textileio/go-textile-core/threadservice"
)

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
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
func newHub() *Hub {
	return &Hub{
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
			data := rec.Value().RawData()
			msg := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
			base64.StdEncoding.Encode(msg, data)
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
