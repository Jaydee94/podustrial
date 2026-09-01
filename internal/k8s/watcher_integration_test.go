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

	// Wait for the initial sync status event. Pre-existing managed pods left over
	// from other tests sharing the "default" namespace may emit machine_added/updated
	// events before or interleaved with it, so skip past those instead of asserting
	// the very first event is cluster_status.
	waitFor := func(deadline time.Duration, match func(factory.Event) bool) factory.Event {
		t.Helper()
		timeout := time.After(deadline)
		for {
			select {
			case ev := <-events:
				if match(ev) {
					return ev
				}
			case <-timeout:
				t.Fatal("timed out waiting for expected event")
			}
		}
	}

	waitFor(5*time.Second, func(ev factory.Event) bool {
		return ev.Type == factory.EventClusterStatus
	})

	pod, err := c.SpawnContainer(ctx, SpawnContainerRequest{ID: "watch-1"})
	if err != nil {
		t.Fatalf("SpawnContainer: %v", err)
	}
	defer c.DeletePod(context.Background(), pod.Name)

	waitFor(10*time.Second, func(ev factory.Event) bool {
		return ev.Type == factory.EventMachineAdded && ev.Machine != nil && ev.Machine.ID == pod.Name
	})
}
