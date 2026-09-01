package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Jaydee94/podustrial/internal/factory"
)

func TestHub_BroadcastsEventToConnectedClient(t *testing.T) {
	hub := NewHub()
	srv := httptest.NewServer(hub)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	events := make(chan factory.Event, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx, events)

	// Wait for the hub to actually register the client via ServeWS before
	// broadcasting, polling instead of guessing a fixed sleep duration.
	deadline := time.Now().Add(2 * time.Second)
	for {
		hub.mu.Lock()
		registered := len(hub.clients) > 0
		hub.mu.Unlock()
		if registered {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for client to register with the hub")
		}
		time.Sleep(5 * time.Millisecond)
	}

	events <- factory.Event{Type: factory.EventMachineAdded, Machine: &factory.Machine{ID: "m1", Status: factory.MachineStatusRunning}}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	if !strings.Contains(string(data), `"id":"m1"`) {
		t.Errorf("message = %s, want it to contain machine id m1", data)
	}
}
