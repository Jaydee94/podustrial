package k8s

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Jaydee94/podustrial/internal/factory"
)

// fakeChecker is safe for concurrent use: Run() reads err via Healthy() on its
// own goroutine while the test mutates it, and signals called after every
// invocation so the test can deterministically wait for a tick to be fully
// processed before sending the next one or changing err.
type fakeChecker struct {
	mu     sync.Mutex
	err    error
	called chan struct{}
}

func (f *fakeChecker) Healthy(ctx context.Context) error {
	f.mu.Lock()
	err := f.err
	f.mu.Unlock()
	f.called <- struct{}{}
	return err
}

func (f *fakeChecker) setErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func TestHealthMonitor_EmitsOnlyOnStatusChange(t *testing.T) {
	checker := &fakeChecker{called: make(chan struct{})}
	out := make(chan factory.Event, 10)
	m := NewHealthMonitor(checker, out)

	ctx, cancel := context.WithCancel(context.Background())
	tick := make(chan time.Time, 1)
	done := make(chan struct{})
	go func() {
		m.Run(ctx, tick)
		close(done)
	}()

	tick <- time.Now() // still OK, no event expected
	<-checker.called
	checker.setErr(errors.New("unreachable"))
	tick <- time.Now() // OK -> down, event expected
	<-checker.called
	tick <- time.Now() // still down, no event expected
	<-checker.called
	checker.setErr(nil)
	tick <- time.Now() // down -> OK, event expected
	<-checker.called

	var events []factory.Event
	timeout := time.After(2 * time.Second)
collect:
	for {
		select {
		case ev := <-out:
			events = append(events, ev)
			if len(events) == 2 {
				break collect
			}
		case <-timeout:
			break collect
		}
	}
	cancel()
	<-done

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2: %+v", len(events), events)
	}
	if events[0].ClusterStatus != factory.ClusterStatusDown {
		t.Errorf("event 0 = %v, want down", events[0].ClusterStatus)
	}
	if events[1].ClusterStatus != factory.ClusterStatusOK {
		t.Errorf("event 1 = %v, want ok", events[1].ClusterStatus)
	}
}
