//go:build integration

package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/Jaydee94/podustrial/internal/factory"
)

func TestWatcher_EmitsEventForNewManagedPod(t *testing.T) {
	c := testClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan factory.Event, 10)
	w := NewWatcher(c, events)
	go w.Run(ctx)

	// wait for initial sync status event
	select {
	case ev := <-events:
		if ev.Type != factory.EventClusterStatus {
			t.Fatalf("expected initial cluster_status event, got %+v", ev)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for initial cluster_status event")
	}

	pod, err := c.SpawnContainer(ctx, SpawnContainerRequest{ID: "watch-1"})
	if err != nil {
		t.Fatalf("SpawnContainer: %v", err)
	}
	defer c.DeletePod(context.Background(), pod.Name)

	select {
	case ev := <-events:
		if ev.Type != factory.EventMachineAdded || ev.Machine == nil || ev.Machine.ID != pod.Name {
			t.Fatalf("expected machine_added for %s, got %+v", pod.Name, ev)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for machine_added event")
	}
}
