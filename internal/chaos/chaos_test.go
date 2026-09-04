package chaos

import (
	"bytes"
	"context"
	"errors"
	"log"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	ch chan time.Time
}

func newFakeClock() *fakeClock {
	// Unbuffered: tick() blocks until Run's select actually receives it, so
	// tests can synchronize on "the tick was consumed" instead of sleeping.
	return &fakeClock{ch: make(chan time.Time)}
}

func (f *fakeClock) After(_ time.Duration) <-chan time.Time {
	return f.ch
}

var fixedTick = time.Unix(0, 0)

func (f *fakeClock) tick() {
	f.ch <- fixedTick
}

type fakeDeleter struct {
	mu        sync.Mutex
	names     []string
	deleted   []string
	listErr   error
	deleteErr error
}

func (f *fakeDeleter) ListManagedPodNames(ctx context.Context) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	return append([]string(nil), f.names...), nil
}

func (f *fakeDeleter) DeletePod(ctx context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, name)
	return nil
}

// captureLog redirects the package-level logger for the duration of the test
// and restores it on cleanup, so tests can assert on what got logged.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &buf
}

func TestService_Disabled_NeverDeletes(t *testing.T) {
	// Run returns immediately when disabled (it never touches the clock),
	// so it can be called synchronously — no goroutine/tick/cancel needed.
	clock := newFakeClock()
	deleter := &fakeDeleter{names: []string{"pod-a"}}
	s := NewService(Config{Enabled: false, Interval: time.Millisecond, Probability: 1.0}, deleter, clock, rand.New(rand.NewSource(1)))

	s.Run(context.Background())

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
	cancel()
	<-done

	deleter.mu.Lock()
	defer deleter.mu.Unlock()
	if len(deleter.deleted) != 0 {
		t.Errorf("expected no deletions at probability 0, got %v", deleter.deleted)
	}
}

func TestNewService_NilRNGDoesNotPanic(t *testing.T) {
	clock := newFakeClock()
	deleter := &fakeDeleter{names: []string{"pod-a"}}
	s := NewService(Config{Enabled: true, Interval: time.Millisecond, Probability: 1.0}, deleter, clock, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()
	clock.tick()
	cancel()
	<-done

	deleter.mu.Lock()
	defer deleter.mu.Unlock()
	if len(deleter.deleted) != 1 {
		t.Fatalf("expected a default rng to be used and exactly 1 deletion, got %v", deleter.deleted)
	}
}

func TestNewService_NilClockDoesNotPanic(t *testing.T) {
	deleter := &fakeDeleter{names: []string{"pod-a"}}
	s := NewService(Config{Enabled: true, Interval: time.Hour, Probability: 1.0}, deleter, nil, rand.New(rand.NewSource(1)))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Run starts so it returns via ctx.Done(), never the real hour-long timer
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return promptly for an already-cancelled context with a default real clock")
	}
}

func TestNewService_NonPositiveIntervalDisablesService(t *testing.T) {
	clock := newFakeClock()
	deleter := &fakeDeleter{names: []string{"pod-a"}}
	s := NewService(Config{Enabled: true, Interval: 0, Probability: 1.0}, deleter, clock, rand.New(rand.NewSource(1)))

	done := make(chan struct{})
	go func() {
		s.Run(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return for a non-positive interval; expected it to be treated as disabled")
	}

	deleter.mu.Lock()
	defer deleter.mu.Unlock()
	if len(deleter.deleted) != 0 {
		t.Errorf("expected non-positive interval to disable the service, got %v", deleter.deleted)
	}
}

func TestService_ListError_LogsAndContinues(t *testing.T) {
	// Regression: ListManagedPodNames' error was silently discarded, so an
	// RBAC misconfiguration or API timeout produced no observable signal
	// anywhere — chaos would just quietly never delete anything.
	buf := captureLog(t)
	clock := newFakeClock()
	deleter := &fakeDeleter{listErr: errors.New("rbac denied")}
	s := NewService(Config{Enabled: true, Interval: time.Millisecond, Probability: 1.0}, deleter, clock, rand.New(rand.NewSource(1)))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()
	clock.tick()
	cancel()
	<-done

	if !strings.Contains(buf.String(), "rbac denied") {
		t.Errorf("expected the list error to be logged, got log output: %q", buf.String())
	}
}

func TestService_DeleteError_LogsAndContinues(t *testing.T) {
	// Regression: DeletePod's error was silently discarded the same way.
	buf := captureLog(t)
	clock := newFakeClock()
	deleter := &fakeDeleter{names: []string{"pod-a"}, deleteErr: errors.New("pod already gone")}
	s := NewService(Config{Enabled: true, Interval: time.Millisecond, Probability: 1.0}, deleter, clock, rand.New(rand.NewSource(1)))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()
	clock.tick()
	cancel()
	<-done

	if !strings.Contains(buf.String(), "pod already gone") {
		t.Errorf("expected the delete error to be logged, got log output: %q", buf.String())
	}
}
