package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Jaydee94/podustrial/internal/factory"
)

const writeTimeout = 5 * time.Second

// The frontend is embedded into the same Go binary and served from the same
// origin as this API (spec: single local process/port), so a same-origin
// check is sufficient here and needs no separate allowlist config. Requests
// without an Origin header (non-browser clients, e.g. tests or CLI tools)
// are allowed since they can't be a cross-site browser attack.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		if err != nil || u.Host != r.Host {
			return false
		}
		expectedScheme := "http"
		if r.TLS != nil {
			expectedScheme = "https"
		}
		return u.Scheme == expectedScheme
	},
}

type Hub struct {
	mu sync.Mutex
	// clients maps each connection to its own write-mutex: gorilla/websocket
	// only supports one concurrent writer per connection, and Broadcast must
	// serialize writes to the same conn even if called from multiple
	// goroutines.
	clients map[*websocket.Conn]*sync.Mutex
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*websocket.Conn]*sync.Mutex)}
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.ServeWS(w, r)
}

// maxClientMessageSize bounds incoming frame size. ServeWS never acts on
// client payloads (it only reads to detect disconnects), so this just
// prevents a client from forcing large allocations via ReadMessage.
const maxClientMessageSize = 512

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(maxClientMessageSize)

	h.mu.Lock()
	h.clients[conn] = &sync.Mutex{}
	h.mu.Unlock()

	defer h.remove(conn)

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *Hub) remove(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
	conn.Close()
}

func (h *Hub) Broadcast(event factory.Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.mu.Lock()
	conns := make(map[*websocket.Conn]*sync.Mutex, len(h.clients))
	for conn, wmu := range h.clients {
		conns[conn] = wmu
	}
	h.mu.Unlock()

	// Write outside h.mu so a slow/unresponsive client can't block
	// registration/unregistration or broadcasts to other clients; the
	// deadline bounds how long a single stuck client can delay this call.
	// Each connection's own write-mutex serializes concurrent writers, since
	// gorilla/websocket only supports one writer per connection at a time.
	for conn, wmu := range conns {
		wmu.Lock()
		conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		err := conn.WriteMessage(websocket.TextMessage, data)
		wmu.Unlock()
		if err != nil {
			h.remove(conn)
		}
	}
}

func (h *Hub) Run(ctx context.Context, events <-chan factory.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			h.Broadcast(ev)
		}
	}
}
