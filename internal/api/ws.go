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
		return err == nil && u.Host == r.Host
	},
}

type Hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*websocket.Conn]struct{})}
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.ServeWS(w, r)
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.clients, conn)
		h.mu.Unlock()
		conn.Close()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *Hub) Broadcast(event factory.Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.mu.Lock()
	conns := make([]*websocket.Conn, 0, len(h.clients))
	for conn := range h.clients {
		conns = append(conns, conn)
	}
	h.mu.Unlock()

	// Write outside the lock so a slow/unresponsive client can't block
	// registration/unregistration or broadcasts to other clients; the
	// deadline bounds how long a single stuck client can delay this call.
	for _, conn := range conns {
		conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		conn.WriteMessage(websocket.TextMessage, data)
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
