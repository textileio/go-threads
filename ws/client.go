// Inspired by https://github.com/gorilla/websocket/tree/master/examples/chat with
// adaptations for multiple rooms ("threads" in Textile parlance) and authentication.
package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mr-tron/base58"
	ma "github.com/multiformats/go-multiaddr"
	sym "github.com/textileio/go-textile-core/crypto/symmetric"
	"github.com/textileio/go-textile-core/thread"
	"github.com/textileio/go-textile-threads/util"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512

	// Duration to wait for a message request to complete.
	messageTimeout = time.Second * 10
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// @todo: auth with follow and/or read key
			return true
		},
	}

	newline = []byte{'\n'}
	space   = []byte{' '}
)

type req struct {
	Type string `json:"type"`
	Msg  string `json:"msg"`
}

type addThreadMsg struct {
	Addr      string `json:"address"`
	FollowKey string `json:"follow_key"`
	ReadKey   string `json:"read_key"`
}

type subscribeMsg struct {
	Threads []string `json:"threads"`
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte

	// Active threads.
	threads map[thread.ID]struct{}
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(
				err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure) {
				log.Errorf("error reading message: %s", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))

		var r req
		if err = json.Unmarshal(message, &r); err != nil {
			c.send <- []byte(err.Error())
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), messageTimeout)
		switch r.Type {
		case "add-thread":
			var m addThreadMsg
			if err = json.Unmarshal([]byte(r.Msg), &m); err != nil {
				c.send <- []byte(err.Error())
				continue
			}

			var info thread.Info
			if m.Addr != "" {
				if m.FollowKey == "" {
					c.send <- []byte("follow key is required with address")
					continue
				}

				fk, err := parseKey(m.FollowKey)
				if err != nil {
					c.send <- []byte(err.Error())
					continue
				}
				var rk *sym.Key
				if m.ReadKey != "" {
					rk, err = parseKey(m.ReadKey)
					if err != nil {
						c.send <- []byte(err.Error())
						continue
					}
				}
				maddr, err := ma.NewMultiaddr(m.Addr)
				if err != nil {
					c.send <- []byte(err.Error())
					continue
				}

				info, err = c.hub.service.AddThread(ctx, maddr, fk, rk)
				if err != nil {
					c.send <- []byte(err.Error())
					continue
				}
			} else {
				info, err = util.CreateThread(c.hub.service, thread.NewIDV1(thread.Raw, 32))
				if err != nil {
					c.send <- []byte(err.Error())
					continue
				}
			}

			log.Debugf("added thread %s", info.ID)

		case "pull-thread":
			c.send <- []byte("todo")
		case "delete-thread":
			c.send <- []byte("todo")
		case "add-follower":
			c.send <- []byte("todo")
		case "add-record":
			c.send <- []byte("todo")
		case "get-record":
			c.send <- []byte("todo")
		case "subscribe":
			var m subscribeMsg
			if err = json.Unmarshal([]byte(r.Msg), &m); err != nil {
				c.send <- []byte(err.Error())
				continue
			}

			for _, t := range m.Threads {
				id, err := thread.Decode(t)
				if err != nil {
					c.send <- []byte(err.Error())
				} else {
					log.Debugf("client requested thread %s", id.String())

					c.threads[id] = struct{}{}
				}
			}
		default:
			c.send <- []byte("invalid message 'type'")
		}

		cancel()
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
// @todo: Handle write errors.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write(newline)
				_, _ = w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWs handles websocket requests from the peer.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error(err)
		return
	}
	client := &Client{
		hub:     hub,
		conn:    conn,
		send:    make(chan []byte, 256),
		threads: make(map[thread.ID]struct{}),
	}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}

func parseKey(k string) (*sym.Key, error) {
	b, err := base58.Decode(k)
	if err != nil {
		return nil, err
	}
	return sym.NewKey(b)
}
