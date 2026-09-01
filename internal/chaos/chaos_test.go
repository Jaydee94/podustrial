package chaos

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	ch chan time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{ch: make(chan time.Time, 8)}
}

func (f *fakeClock) After(_ time.Duration) <-chan time.Time {
	return f.ch
}

func (f *fakeClock) tick() {
	f.ch <- time.Now()
}

type fakeDeleter struct {
	mu      sync.Mutex
	names   []string
	deleted []string
}

func (f *fakeDeleter) ListManagedPodNames(ctx context.Context) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.names...), nil
}

func (f *fakeDeleter) DeletePod(ctx context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, name)
	return nil
}

func TestService_Disabled_NeverDeletes(t *testing.T) {
	clock := newFakeClock()
	deleter := &fakeDeleter{names: []string{"pod-a"}}
	s := NewService(Config{Enabled: false, Interval: time.Millisecond, Probability: 1.0}, deleter, clock, rand.New(rand.NewSource(1)))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()
	clock.tick()
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	deleter.mu.Lock()
	defer deleter.mu.Unlock()
	if len(deleter.deleted) != 0 {
		t.Errorf("expected no deletions while disabled, got %v", deleter.deleted)
	}
}

func TestService_ProbabilityOne_DeletesOnEveryTick(t *testing.T) {
	clock := newFakeClock()
	deleter := &fakeDeleter{names: []string{"pod-a", "pod-b"}}
	s := NewService(Config{Enabled: true, Interval: time.Millisecond, Probability: 1.0}, deleter, clock, rand.New(rand.NewSource(1)))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()
	clock.tick()
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	deleter.mu.Lock()
	defer deleter.mu.Unlock()
	if len(deleter.deleted) != 1 {
		t.Fatalf("expected exactly 1 deletion, got %v", deleter.deleted)
	}
	if deleter.deleted[0] != "pod-a" && deleter.deleted[0] != "pod-b" {
		t.Errorf("deleted unexpected pod: %s", deleter.deleted[0])
	}
}

func TestService_ProbabilityZero_NeverDeletes(t *testing.T) {
	clock := newFakeClock()
	deleter := &fakeDeleter{names: []string{"pod-a"}}
	s := NewService(Config{Enabled: true, Interval: time.Millisecond, Probability: 0.0}, deleter, clock, rand.New(rand.NewSource(1)))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()
	clock.tick()
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	deleter.mu.Lock()
	defer deleter.mu.Unlock()
	if len(deleter.deleted) != 0 {
		t.Errorf("expected no deletions at probability 0, got %v", deleter.deleted)
	}
}
