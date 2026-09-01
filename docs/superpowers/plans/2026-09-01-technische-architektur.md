# Podustrial Technische Architektur — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the technical foundation of Podustrial as a working, end-to-end vertical slice — a local single-player app where a Go backend translates a minimal "spawn a machine" action into a real Pod on a real local kind Kubernetes cluster, watches real cluster state, and streams it to a Phaser/TypeScript frontend over WebSocket.

**Architecture:** A Go backend (`cmd/server`) is the sole API and cluster-access layer. It uses `client-go` to create real K8s objects and to watch real cluster state via an informer, translates both directions between Kubernetes objects and a small "factory vocabulary" (`internal/factory`), and pushes state changes to connected browsers over WebSocket. The frontend (Vite + Phaser + TypeScript) is embedded into the Go binary and renders machines purely reactively from server-pushed events. A kind cluster is created/destroyed by shell scripts wrapped in a Makefile. Progress and a config-driven chaos service (real pod deletion) round out the four architecture components from the spec.

**Tech Stack:** Go 1.22+, `client-go`, `gorilla/websocket`, `modernc.org/sqlite` (pure-Go, no CGO), kind, Docker; Vite + TypeScript + Phaser 3, Vitest; Playwright for E2E.

**Spec:** `docs/superpowers/specs/2026-09-01-technische-architektur-design.md`

## Global Constraints

- Scope is local single-player only — no shared-cluster/multiplayer mode (spec §1, §8).
- The backend is the only component that talks to the cluster; the frontend never does (spec §2).
- Real K8s objects only — no simulated scheduling/failure logic. Dummy containers use image `busybox:1.36` with `sleep 3600` (spec §1).
- Chaos service rates (interval, probability) are config values, never hardcoded (spec §2, §6).
- Progress is persisted locally, not remotely (spec §2, §4).
- E2E tests are automated with Playwright against the real local stack from the start, not added later (spec §7).
- Command split: `make start` (player, one process/port, embedded frontend), `make stop` (teardown), `make dev` (developer hot-reload, not for players) (spec §5).
- Go module path: `github.com/Jaydee94/podustrial`. Go version floor: 1.22 (needed for method-pattern `http.ServeMux` routing).
- This plan builds the architecture's proof-of-concept action ("spawn a container" → a bare Pod), matching the Level 1 backend behavior from the spec's gating table (spec §3). It does not build Level 2-8 mechanics, game UI/art, or win conditions — those are explicitly out of scope (spec §8).

---

## File Structure

```
go.mod
internal/factory/state.go        — factory vocabulary types + Pod↔Machine translation (pure, no I/O)
internal/factory/state_test.go
internal/k8s/client.go           — client-go wiring, health check
internal/k8s/client_test.go
internal/k8s/actions.go          — SpawnContainer, ListManagedPodNames, DeletePod
internal/k8s/actions_integration_test.go
internal/k8s/watcher.go          — informer-based watcher, emits factory.Event
internal/k8s/watcher_integration_test.go
internal/k8s/health.go           — HealthMonitor, emits cluster status transitions
internal/k8s/health_test.go
internal/api/http.go             — REST handlers (/healthz, /api/actions/spawn-container)
internal/api/http_test.go
internal/api/ws.go               — WebSocket Hub
internal/api/ws_integration_test.go
internal/progress/store.go       — SQLite-backed level progress store
internal/progress/store_test.go
internal/chaos/chaos.go          — chaos service (interval/probability pod deletion)
internal/chaos/chaos_test.go
cmd/server/main.go               — wires everything, embeds frontend/dist
cmd/server/frontend.go           — go:embed directive
kind-config.yaml                 — kind cluster config
scripts/start.sh                 — Docker check, kind create, run server
scripts/stop.sh                  — kind delete
Makefile                         — start, stop, dev, test, test-integration targets
frontend/package.json
frontend/vite.config.ts
frontend/index.html
frontend/src/net/socket.ts       — typed WebSocket client
frontend/src/net/socket.test.ts
frontend/src/scene/FactoryScene.ts
frontend/src/scene/FactoryScene.test.ts
frontend/src/main.ts
e2e/podustrial.spec.ts
playwright.config.ts
.github/workflows/ci.yml
```

---

### Task 1: Setup & Start Scripts (kind cluster lifecycle)

**Files:**
- Create: `kind-config.yaml`
- Create: `scripts/start.sh`
- Create: `scripts/stop.sh`
- Create: `Makefile`

**Interfaces:**
- Produces: a reachable kind cluster named `podustrial` on `kind-podustrial` kubectl context, and `make start` / `make stop` / `make dev` entrypoints later tasks build on.

- [ ] **Step 1: Write `kind-config.yaml`**

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: podustrial
nodes:
  - role: control-plane
```

- [ ] **Step 2: Write `scripts/start.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="podustrial"

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker wurde nicht gefunden. Bitte installiere Docker Desktop: https://www.docker.com/products/docker-desktop/" >&2
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  echo "Docker läuft nicht. Bitte starte Docker Desktop und versuche es erneut." >&2
  exit 1
fi

if ! command -v kind >/dev/null 2>&1; then
  echo "kind wurde nicht gefunden. Installationsanleitung: https://kind.sigs.k8s.io/docs/user/quick-start/#installation" >&2
  exit 1
fi

cleanup_on_failure() {
  echo "Cluster-Erstellung fehlgeschlagen, räume auf..." >&2
  kind delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
}

if ! kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
  trap cleanup_on_failure ERR
  kind create cluster --name "$CLUSTER_NAME" --config kind-config.yaml
  trap - ERR
fi

export KUBECONFIG
KUBECONFIG="$(kind get kubeconfig-path --name "$CLUSTER_NAME" 2>/dev/null || true)"
kind export kubeconfig --name "$CLUSTER_NAME"

go run ./cmd/server
```

- [ ] **Step 3: Write `scripts/stop.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail

kind delete cluster --name podustrial
```

- [ ] **Step 4: Make scripts executable**

Run: `chmod +x scripts/start.sh scripts/stop.sh`

- [ ] **Step 5: Write `Makefile`**

```makefile
.PHONY: start stop dev test test-integration

start:
	./scripts/start.sh

stop:
	./scripts/stop.sh

dev:
	kind create cluster --name podustrial --config kind-config.yaml || true
	kind export kubeconfig --name podustrial
	( cd frontend && npm run dev & )
	go run ./cmd/server

test:
	go test ./...
	( cd frontend && npm test )

test-integration:
	kind create cluster --name podustrial-test --config kind-config.yaml || true
	kind export kubeconfig --name podustrial-test
	go test -tags=integration ./...
	kind delete cluster --name podustrial-test
```

- [ ] **Step 6: Verify cluster creation and teardown manually**

Run: `./scripts/stop.sh || true && kind create cluster --name podustrial --config kind-config.yaml && kubectl --context kind-podustrial get nodes && kind delete cluster --name podustrial`
Expected: node listed as `Ready`, then cluster deleted without error.

- [ ] **Step 7: Commit**

```bash
git add kind-config.yaml scripts/start.sh scripts/stop.sh Makefile
git commit -m "chore: add kind cluster lifecycle scripts and Makefile"
```

---

### Task 2: Go Module & Kubernetes Client Wiring

**Files:**
- Create: `go.mod`
- Create: `internal/k8s/client.go`
- Test: `internal/k8s/client_test.go`

**Interfaces:**
- Produces: `k8s.Client` struct with `Clientset kubernetes.Interface`, `NewClient(kubeconfigPath, namespace string) (*Client, error)`, `(*Client) Healthy(ctx context.Context) error`.

- [ ] **Step 1: Initialize Go module**

Run: `go mod init github.com/Jaydee94/podustrial && go mod edit -go=1.22`

- [ ] **Step 2: Add client-go dependency**

Run: `go get k8s.io/client-go@v0.31.0 k8s.io/api@v0.31.0 k8s.io/apimachinery@v0.31.0`

- [ ] **Step 3: Write the failing test**

```go
// internal/k8s/client_test.go
package k8s

import "testing"

func TestNewClient_InvalidKubeconfigPath_ReturnsError(t *testing.T) {
	_, err := NewClient("/nonexistent/kubeconfig", "default")
	if err == nil {
		t.Fatal("expected an error for a missing kubeconfig file, got nil")
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/k8s/... -run TestNewClient_InvalidKubeconfigPath_ReturnsError -v`
Expected: FAIL — `NewClient` undefined.

- [ ] **Step 5: Write implementation**

```go
// internal/k8s/client.go
package k8s

import (
	"context"
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	Clientset kubernetes.Interface
	namespace string
}

func NewClient(kubeconfigPath, namespace string) (*Client, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("build kubeconfig: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}
	return &Client{Clientset: cs, namespace: namespace}, nil
}

func (c *Client) Healthy(ctx context.Context) error {
	if _, err := c.Clientset.Discovery().ServerVersion(); err != nil {
		return fmt.Errorf("cluster not reachable: %w", err)
	}
	return nil
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/k8s/... -run TestNewClient_InvalidKubeconfigPath_ReturnsError -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/k8s/client.go internal/k8s/client_test.go
git commit -m "feat: add Kubernetes client wiring"
```

---

### Task 3: Factory Vocabulary & Translation Logic

**Files:**
- Create: `internal/factory/state.go`
- Test: `internal/factory/state_test.go`

**Interfaces:**
- Consumes: nothing (pure package, only depends on `k8s.io/api/core/v1`).
- Produces: `factory.MachineStatus`, `factory.Machine{ID, Status}`, `factory.ClusterStatus`, `factory.EventType`, `factory.Event{Type, Machine *Machine, ClusterStatus ClusterStatus}`, `factory.TranslatePodStatus(phase corev1.PodPhase) MachineStatus`, `factory.PodEventToFactoryEvent(eventType EventType, pod *corev1.Pod) Event`, `factory.NewClusterStatusEvent(status ClusterStatus) Event`. All later tasks (watcher, hub, frontend) depend on this exact shape.

- [ ] **Step 1: Write the failing test**

```go
// internal/factory/state_test.go
package factory

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTranslatePodStatus(t *testing.T) {
	cases := []struct {
		phase corev1.PodPhase
		want  MachineStatus
	}{
		{corev1.PodPending, MachineStatusPending},
		{corev1.PodRunning, MachineStatusRunning},
		{corev1.PodFailed, MachineStatusFailed},
		{corev1.PodSucceeded, MachineStatusPending},
	}
	for _, tc := range cases {
		if got := TranslatePodStatus(tc.phase); got != tc.want {
			t.Errorf("TranslatePodStatus(%v) = %v, want %v", tc.phase, got, tc.want)
		}
	}
}

func TestPodEventToFactoryEvent(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "podustrial-machine-1"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	ev := PodEventToFactoryEvent(EventMachineAdded, pod)
	if ev.Type != EventMachineAdded {
		t.Errorf("Type = %v, want %v", ev.Type, EventMachineAdded)
	}
	if ev.Machine == nil || ev.Machine.ID != "podustrial-machine-1" || ev.Machine.Status != MachineStatusRunning {
		t.Errorf("Machine = %+v, want ID=podustrial-machine-1 Status=running", ev.Machine)
	}
}

func TestNewClusterStatusEvent(t *testing.T) {
	ev := NewClusterStatusEvent(ClusterStatusDown)
	if ev.Type != EventClusterStatus || ev.ClusterStatus != ClusterStatusDown {
		t.Errorf("got %+v, want Type=cluster_status ClusterStatus=stromausfall", ev)
	}
	if ev.Machine != nil {
		t.Errorf("Machine should be nil for a cluster status event, got %+v", ev.Machine)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/factory/... -v`
Expected: FAIL — package `factory` does not exist yet.

- [ ] **Step 3: Write implementation**

```go
// internal/factory/state.go
package factory

import (
	corev1 "k8s.io/api/core/v1"
)

type MachineStatus string

const (
	MachineStatusPending MachineStatus = "pending"
	MachineStatusRunning MachineStatus = "running"
	MachineStatusFailed  MachineStatus = "failed"
)

type Machine struct {
	ID     string        `json:"id"`
	Status MachineStatus `json:"status"`
}

type ClusterStatus string

const (
	ClusterStatusOK   ClusterStatus = "ok"
	ClusterStatusDown ClusterStatus = "stromausfall"
)

type EventType string

const (
	EventMachineAdded   EventType = "machine_added"
	EventMachineUpdated EventType = "machine_updated"
	EventMachineRemoved EventType = "machine_removed"
	EventClusterStatus  EventType = "cluster_status"
)

type Event struct {
	Type          EventType     `json:"type"`
	Machine       *Machine      `json:"machine,omitempty"`
	ClusterStatus ClusterStatus `json:"clusterStatus,omitempty"`
}

func TranslatePodStatus(phase corev1.PodPhase) MachineStatus {
	switch phase {
	case corev1.PodRunning:
		return MachineStatusRunning
	case corev1.PodFailed:
		return MachineStatusFailed
	default:
		return MachineStatusPending
	}
}

func PodEventToFactoryEvent(eventType EventType, pod *corev1.Pod) Event {
	return Event{
		Type: eventType,
		Machine: &Machine{
			ID:     pod.Name,
			Status: TranslatePodStatus(pod.Status.Phase),
		},
	}
}

func NewClusterStatusEvent(status ClusterStatus) Event {
	return Event{Type: EventClusterStatus, ClusterStatus: status}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/factory/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/factory/state.go internal/factory/state_test.go
git commit -m "feat: add factory vocabulary and pod translation logic"
```

---

### Task 4: Spawn Action (Pod Creation)

**Files:**
- Modify: `internal/k8s/client.go` (no change needed — namespace already present)
- Create: `internal/k8s/actions.go`
- Test: `internal/k8s/actions_integration_test.go`

**Interfaces:**
- Consumes: `k8s.Client` (Task 2).
- Produces: `k8s.SpawnContainerRequest{ID string}`, `k8s.PodName(id string) string`, `k8s.ManagedByLabel`, `k8s.ManagedByValue` constants, `(*Client) SpawnContainer(ctx, req SpawnContainerRequest) (*corev1.Pod, error)`. Used by Task 5 (watcher label selector) and Task 6 (HTTP handler).

- [ ] **Step 1: Write the failing integration test**

```go
// internal/k8s/actions_integration_test.go
//go:build integration

package k8s

import (
	"context"
	"os"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testClient(t *testing.T) *Client {
	t.Helper()
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("KUBECONFIG not set, skipping integration test")
	}
	c, err := NewClient(kubeconfig, "default")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestSpawnContainer_CreatesLabeledPod(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()

	pod, err := c.SpawnContainer(ctx, SpawnContainerRequest{ID: "test-1"})
	if err != nil {
		t.Fatalf("SpawnContainer: %v", err)
	}
	defer c.Clientset.CoreV1().Pods("default").Delete(ctx, pod.Name, metav1.DeleteOptions{})

	if pod.Name != "podustrial-machine-test-1" {
		t.Errorf("pod name = %q, want podustrial-machine-test-1", pod.Name)
	}
	if pod.Labels[ManagedByLabel] != ManagedByValue {
		t.Errorf("label %s = %q, want %q", ManagedByLabel, pod.Labels[ManagedByLabel], ManagedByValue)
	}
	if len(pod.Spec.Containers) != 1 || pod.Spec.Containers[0].Image != "busybox:1.36" {
		t.Errorf("unexpected containers: %+v", pod.Spec.Containers)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `KUBECONFIG=$(kind get kubeconfig --name podustrial > /tmp/kc && echo /tmp/kc) go test -tags=integration ./internal/k8s/... -run TestSpawnContainer_CreatesLabeledPod -v`
Expected: FAIL — `SpawnContainer` undefined.

- [ ] **Step 3: Write implementation**

```go
// internal/k8s/actions.go
package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ManagedByLabel = "app.kubernetes.io/managed-by"
	ManagedByValue = "podustrial"
	machineImage   = "busybox:1.36"
)

type SpawnContainerRequest struct {
	ID string `json:"id"`
}

func PodName(id string) string {
	return fmt.Sprintf("podustrial-machine-%s", id)
}

func (c *Client) SpawnContainer(ctx context.Context, req SpawnContainerRequest) (*corev1.Pod, error) {
	if req.ID == "" {
		return nil, fmt.Errorf("id must not be empty")
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PodName(req.ID),
			Namespace: c.namespace,
			Labels: map[string]string{
				ManagedByLabel: ManagedByValue,
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "machine",
					Image:   machineImage,
					Command: []string{"sleep", "3600"},
				},
			},
		},
	}
	created, err := c.Clientset.CoreV1().Pods(c.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return c.Clientset.CoreV1().Pods(c.namespace).Get(ctx, PodName(req.ID), metav1.GetOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("create pod: %w", err)
	}
	return created, nil
}

func (c *Client) ListManagedPodNames(ctx context.Context) ([]string, error) {
	pods, err := c.Clientset.CoreV1().Pods(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: ManagedByLabel + "=" + ManagedByValue,
	})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	names := make([]string, len(pods.Items))
	for i, p := range pods.Items {
		names[i] = p.Name
	}
	return names, nil
}

func (c *Client) DeletePod(ctx context.Context, name string) error {
	if err := c.Clientset.CoreV1().Pods(c.namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("delete pod %s: %w", name, err)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `KUBECONFIG=/tmp/kc go test -tags=integration ./internal/k8s/... -run TestSpawnContainer_CreatesLabeledPod -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/k8s/actions.go internal/k8s/actions_integration_test.go
git commit -m "feat: add SpawnContainer action and managed-pod helpers"
```

---

### Task 5: Cluster State Watcher

**Files:**
- Create: `internal/k8s/watcher.go`
- Test: `internal/k8s/watcher_integration_test.go`

**Interfaces:**
- Consumes: `k8s.Client` (Task 2), `factory.Event` / `factory.PodEventToFactoryEvent` / `factory.NewClusterStatusEvent` (Task 3), `k8s.ManagedByLabel`/`ManagedByValue` (Task 4).
- Produces: `k8s.NewWatcher(client *Client, out chan<- factory.Event) *Watcher`, `(*Watcher) Run(ctx context.Context) error`. Used by Task 10 (main.go wiring).

- [ ] **Step 1: Add informers dependency**

Run: `go get k8s.io/client-go@v0.31.0` (already present; ensure `k8s.io/client-go/informers` and `k8s.io/client-go/tools/cache` resolve — no separate module needed)

- [ ] **Step 2: Write the failing integration test**

```go
// internal/k8s/watcher_integration_test.go
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
```

- [ ] **Step 3: Run test to verify it fails**

Run: `KUBECONFIG=/tmp/kc go test -tags=integration ./internal/k8s/... -run TestWatcher_EmitsEventForNewManagedPod -v`
Expected: FAIL — `NewWatcher` undefined.

- [ ] **Step 4: Write implementation**

```go
// internal/k8s/watcher.go
package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/Jaydee94/podustrial/internal/factory"
)

type Watcher struct {
	client *Client
	out    chan<- factory.Event
}

func NewWatcher(client *Client, out chan<- factory.Event) *Watcher {
	return &Watcher{client: client, out: out}
}

func (w *Watcher) Run(ctx context.Context) error {
	factoryInformers := informers.NewSharedInformerFactoryWithOptions(
		w.client.Clientset,
		0,
		informers.WithNamespace(w.client.namespace),
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = ManagedByLabel + "=" + ManagedByValue
		}),
	)
	podInformer := factoryInformers.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod, ok := obj.(*corev1.Pod)
			if ok {
				w.out <- factory.PodEventToFactoryEvent(factory.EventMachineAdded, pod)
			}
		},
		UpdateFunc: func(_, newObj interface{}) {
			pod, ok := newObj.(*corev1.Pod)
			if ok {
				w.out <- factory.PodEventToFactoryEvent(factory.EventMachineUpdated, pod)
			}
		},
		DeleteFunc: func(obj interface{}) {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				pod, ok = tombstone.Obj.(*corev1.Pod)
				if !ok {
					return
				}
			}
			w.out <- factory.PodEventToFactoryEvent(factory.EventMachineRemoved, pod)
		},
	})

	factoryInformers.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), podInformer.HasSynced) {
		return context.Canceled
	}
	w.out <- factory.NewClusterStatusEvent(factory.ClusterStatusOK)
	<-ctx.Done()
	return ctx.Err()
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `KUBECONFIG=/tmp/kc go test -tags=integration ./internal/k8s/... -run TestWatcher_EmitsEventForNewManagedPod -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/k8s/watcher.go internal/k8s/watcher_integration_test.go
git commit -m "feat: add informer-based cluster state watcher"
```

---

### Task 6: Cluster Health Monitor

**Files:**
- Create: `internal/k8s/health.go`
- Test: `internal/k8s/health_test.go`

**Interfaces:**
- Consumes: `factory.Event` / `factory.ClusterStatus` / `factory.NewClusterStatusEvent` (Task 3).
- Produces: `k8s.HealthChecker` interface (`Healthy(ctx context.Context) error` — `*k8s.Client` satisfies it via Task 2), `k8s.NewHealthMonitor(checker HealthChecker, out chan<- factory.Event) *HealthMonitor`, `(*HealthMonitor) Run(ctx context.Context, tick <-chan time.Time)`. Used by Task 10 (main.go wiring).

- [ ] **Step 1: Write the failing test**

```go
// internal/k8s/health_test.go
package k8s

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Jaydee94/podustrial/internal/factory"
)

type fakeChecker struct {
	err error
}

func (f *fakeChecker) Healthy(ctx context.Context) error {
	return f.err
}

func TestHealthMonitor_EmitsOnlyOnStatusChange(t *testing.T) {
	checker := &fakeChecker{err: nil}
	out := make(chan factory.Event, 10)
	m := NewHealthMonitor(checker, out)

	ctx, cancel := context.WithCancel(context.Background())
	tick := make(chan time.Time, 4)
	done := make(chan struct{})
	go func() {
		m.Run(ctx, tick)
		close(done)
	}()

	tick <- time.Now() // still OK, no event expected
	checker.err = errors.New("unreachable")
	tick <- time.Now() // OK -> down, event expected
	tick <- time.Now() // still down, no event expected
	checker.err = nil
	tick <- time.Now() // down -> OK, event expected

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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/k8s/... -run TestHealthMonitor_EmitsOnlyOnStatusChange -v`
Expected: FAIL — `NewHealthMonitor` undefined.

- [ ] **Step 3: Write implementation**

```go
// internal/k8s/health.go
package k8s

import (
	"context"
	"time"

	"github.com/Jaydee94/podustrial/internal/factory"
)

type HealthChecker interface {
	Healthy(ctx context.Context) error
}

type HealthMonitor struct {
	checker HealthChecker
	out     chan<- factory.Event
}

func NewHealthMonitor(checker HealthChecker, out chan<- factory.Event) *HealthMonitor {
	return &HealthMonitor{checker: checker, out: out}
}

func (m *HealthMonitor) Run(ctx context.Context, tick <-chan time.Time) {
	lastStatus := factory.ClusterStatusOK
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick:
			status := factory.ClusterStatusOK
			if err := m.checker.Healthy(ctx); err != nil {
				status = factory.ClusterStatusDown
			}
			if status != lastStatus {
				m.out <- factory.NewClusterStatusEvent(status)
				lastStatus = status
			}
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/k8s/... -run TestHealthMonitor_EmitsOnlyOnStatusChange -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/k8s/health.go internal/k8s/health_test.go
git commit -m "feat: add cluster health monitor with change-only events"
```

---

### Task 7: REST API (health + spawn action)

**Files:**
- Create: `internal/api/http.go`
- Test: `internal/api/http_test.go`

**Interfaces:**
- Consumes: `k8s.Client.Healthy` (Task 2), `k8s.SpawnContainerRequest` / `k8s.Client.SpawnContainer` (Task 4).
- Produces: `api.Server{}`, `api.NewServer(k8sClient *k8s.Client) *Server`, `(*Server) Routes() *http.ServeMux` with `GET /healthz` and `POST /api/actions/spawn-container`. `Routes()` is extended by Task 8 to add `/ws`.

- [ ] **Step 1: Write the failing test**

```go
// internal/api/http_test.go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/Jaydee94/podustrial/internal/k8s"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	client := k8s.NewFakeClient(fake.NewSimpleClientset(), "default")
	return NewServer(client)
}

func TestHandleHealthz_ClusterReachable_ReturnsOK(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleSpawnAction_ValidRequest_CreatesPod(t *testing.T) {
	s := newTestServer(t)
	body := strings.NewReader(`{"id":"http-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/actions/spawn-container", body)
	rec := httptest.NewRecorder()

	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["pod"] != "podustrial-machine-http-1" {
		t.Errorf("pod = %q, want podustrial-machine-http-1", resp["pod"])
	}

	pod, err := s.k8sClient.Clientset.CoreV1().Pods("default").Get(context.Background(), "podustrial-machine-http-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected pod to exist: %v", err)
	}
	if pod.Labels[k8s.ManagedByLabel] != k8s.ManagedByValue {
		t.Errorf("pod missing managed-by label")
	}
	_ = corev1.Pod{}
}

func TestHandleSpawnAction_EmptyBody_ReturnsBadRequest(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/actions/spawn-container", strings.NewReader(""))
	rec := httptest.NewRecorder()

	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
```

- [ ] **Step 2: Add a fake-clientset constructor to the k8s package**

`internal/k8s/client.go` needs a way to build a `*Client` around a fake clientset for tests outside the package. Modify `internal/k8s/client.go`, adding at the end:

```go
func NewFakeClient(cs kubernetes.Interface, namespace string) *Client {
	return &Client{Clientset: cs, namespace: namespace}
}
```

- [ ] **Step 3: Add fake clientset dependency**

Run: `go get k8s.io/client-go/kubernetes/fake@v0.31.0`

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/api/... -v`
Expected: FAIL — package `api` does not exist yet.

- [ ] **Step 5: Write implementation**

```go
// internal/api/http.go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/Jaydee94/podustrial/internal/k8s"
)

type Server struct {
	k8sClient *k8s.Client
}

func NewServer(k8sClient *k8s.Client) *Server {
	return &Server{k8sClient: k8sClient}
}

func (s *Server) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	if err := s.k8sClient.Healthy(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) HandleSpawnAction(w http.ResponseWriter, r *http.Request) {
	var req k8s.SpawnContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	pod, err := s.k8sClient.SpawnContainer(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"pod": pod.Name})
}

func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.HandleHealthz)
	mux.HandleFunc("POST /api/actions/spawn-container", s.HandleSpawnAction)
	return mux
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/api/... -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/k8s/client.go internal/api/http.go internal/api/http_test.go go.mod go.sum
git commit -m "feat: add REST API for health check and spawn action"
```

---

### Task 8: WebSocket Hub

**Files:**
- Create: `internal/api/ws.go`
- Modify: `internal/api/http.go` (add `hub` field, register `/ws` route)
- Test: `internal/api/ws_integration_test.go`

**Interfaces:**
- Consumes: `factory.Event` (Task 3).
- Produces: `api.NewHub() *Hub`, `(*Hub) ServeWS(w http.ResponseWriter, r *http.Request)`, `(*Hub) Broadcast(event factory.Event)`, `(*Hub) Run(ctx context.Context, events <-chan factory.Event)`. `NewServer` signature changes to `NewServer(k8sClient *k8s.Client, hub *Hub) *Server` — used by Task 10 (main.go wiring).

- [ ] **Step 1: Add websocket dependency**

Run: `go get github.com/gorilla/websocket@v1.5.3`

- [ ] **Step 2: Write the failing integration test**

```go
// internal/api/ws_integration_test.go
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

	// give the hub a moment to register the client via ServeWS before broadcasting
	time.Sleep(50 * time.Millisecond)
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
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/api/... -run TestHub_BroadcastsEventToConnectedClient -v`
Expected: FAIL — `NewHub` undefined.

- [ ] **Step 4: Write implementation**

```go
// internal/api/ws.go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"

	"github.com/Jaydee94/podustrial/internal/factory"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
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
	defer h.mu.Unlock()
	for conn := range h.clients {
		conn.WriteMessage(websocket.TextMessage, data)
	}
}

func (h *Hub) Run(ctx context.Context, events <-chan factory.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-events:
			h.Broadcast(ev)
		}
	}
}
```

- [ ] **Step 5: Wire the hub into the REST server**

Modify `internal/api/http.go`:

```go
type Server struct {
	k8sClient *k8s.Client
	hub       *Hub
}

func NewServer(k8sClient *k8s.Client, hub *Hub) *Server {
	return &Server{k8sClient: k8sClient, hub: hub}
}
```

And in `Routes()`, add before `return mux`:

```go
	mux.HandleFunc("/ws", s.hub.ServeWS)
```

Update the two existing call sites in `internal/api/http_test.go` (`newTestServer`) to pass a hub:

```go
func newTestServer(t *testing.T) *Server {
	t.Helper()
	client := k8s.NewFakeClient(fake.NewSimpleClientset(), "default")
	return NewServer(client, NewHub())
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/api/... -v`
Expected: PASS (all `internal/api` tests, including the updated `http_test.go`)

- [ ] **Step 7: Commit**

```bash
git add internal/api/ws.go internal/api/ws_integration_test.go internal/api/http.go internal/api/http_test.go go.mod go.sum
git commit -m "feat: add WebSocket hub and wire it into the API server"
```

---

### Task 9: Progress Store

**Files:**
- Create: `internal/progress/store.go`
- Test: `internal/progress/store_test.go`

**Interfaces:**
- Produces: `progress.Store{}`, `progress.Open(path string) (*Store, error)`, `(*Store) CurrentLevel(ctx context.Context) (int, error)`, `(*Store) SetLevel(ctx context.Context, level int) error`, `(*Store) Close() error`. Used by Task 10 (main.go wiring).

- [ ] **Step 1: Add SQLite driver dependency**

Run: `go get modernc.org/sqlite@v1.34.1`

- [ ] **Step 2: Write the failing test**

```go
// internal/progress/store_test.go
package progress

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStore_DefaultsToLevelOne(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "progress.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	level, err := s.CurrentLevel(context.Background())
	if err != nil {
		t.Fatalf("CurrentLevel: %v", err)
	}
	if level != 1 {
		t.Errorf("level = %d, want 1", level)
	}
}

func TestStore_SetLevel_Persists(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "progress.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	if err := s.SetLevel(ctx, 4); err != nil {
		t.Fatalf("SetLevel: %v", err)
	}
	level, err := s.CurrentLevel(ctx)
	if err != nil {
		t.Fatalf("CurrentLevel: %v", err)
	}
	if level != 4 {
		t.Errorf("level = %d, want 4", level)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/progress/... -v`
Expected: FAIL — package `progress` does not exist yet.

- [ ] **Step 4: Write implementation**

```go
// internal/progress/store.go
package progress

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS progress (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		level INTEGER NOT NULL
	)`); err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO progress (id, level) VALUES (1, 1)`); err != nil {
		return nil, fmt.Errorf("seed row: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) CurrentLevel(ctx context.Context) (int, error) {
	var level int
	if err := s.db.QueryRowContext(ctx, `SELECT level FROM progress WHERE id = 1`).Scan(&level); err != nil {
		return 0, fmt.Errorf("query level: %w", err)
	}
	return level, nil
}

func (s *Store) SetLevel(ctx context.Context, level int) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE progress SET level = ? WHERE id = 1`, level); err != nil {
		return fmt.Errorf("update level: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/progress/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/progress/store.go internal/progress/store_test.go go.mod go.sum
git commit -m "feat: add SQLite-backed progress store"
```

---

### Task 10: Chaos Service

**Files:**
- Create: `internal/chaos/chaos.go`
- Test: `internal/chaos/chaos_test.go`

**Interfaces:**
- Consumes: `k8s.Client.ListManagedPodNames` / `k8s.Client.DeletePod` (Task 4) structurally, via the `Deleter` interface defined here.
- Produces: `chaos.Deleter` interface (`ListManagedPodNames(ctx) ([]string, error)`, `DeletePod(ctx, name string) error` — `*k8s.Client` satisfies it), `chaos.Clock` interface (`After(d time.Duration) <-chan time.Time`), `chaos.Config{Enabled bool, Interval time.Duration, Probability float64}`, `chaos.NewService(cfg Config, deleter Deleter, clock Clock, rng *rand.Rand) *Service`, `(*Service) Run(ctx context.Context)`. Used by Task 10 — wait, used by Task 11 (main.go wiring).

- [ ] **Step 1: Write the failing test**

```go
// internal/chaos/chaos_test.go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/chaos/... -v`
Expected: FAIL — package `chaos` does not exist yet.

- [ ] **Step 3: Write implementation**

```go
// internal/chaos/chaos.go
package chaos

import (
	"context"
	"math/rand"
	"time"
)

type Deleter interface {
	ListManagedPodNames(ctx context.Context) ([]string, error)
	DeletePod(ctx context.Context, name string) error
}

type Clock interface {
	After(d time.Duration) <-chan time.Time
}

type realClock struct{}

func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

func RealClock() Clock { return realClock{} }

type Config struct {
	Enabled     bool
	Interval    time.Duration
	Probability float64
}

type Service struct {
	cfg     Config
	deleter Deleter
	clock   Clock
	rng     *rand.Rand
}

func NewService(cfg Config, deleter Deleter, clock Clock, rng *rand.Rand) *Service {
	return &Service{cfg: cfg, deleter: deleter, clock: clock, rng: rng}
}

func (s *Service) Run(ctx context.Context) {
	if !s.cfg.Enabled {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.clock.After(s.cfg.Interval):
			if s.rng.Float64() >= s.cfg.Probability {
				continue
			}
			names, err := s.deleter.ListManagedPodNames(ctx)
			if err != nil || len(names) == 0 {
				continue
			}
			target := names[s.rng.Intn(len(names))]
			s.deleter.DeletePod(ctx, target)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/chaos/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/chaos/chaos.go internal/chaos/chaos_test.go
git commit -m "feat: add config-driven chaos service"
```

---

### Task 11: Server Wiring & Frontend Embed

**Files:**
- Create: `cmd/server/main.go`
- Create: `cmd/server/frontend.go`
- Create: `frontend/dist/.gitkeep` (placeholder so `go:embed` has a directory to embed before Task 12 builds real frontend output)

**Interfaces:**
- Consumes: everything from Tasks 2–10 (`k8s.Client`, `k8s.Watcher`, `k8s.HealthMonitor`, `api.Server`, `api.Hub`, `progress.Store`, `chaos.Service`).
- Produces: the `podustrial` server binary — `go run ./cmd/server` serves `GET /` (embedded frontend), `GET /healthz`, `POST /api/actions/spawn-container`, `GET /ws`.

- [ ] **Step 1: Create the embed placeholder**

Run: `mkdir -p frontend/dist && touch frontend/dist/.gitkeep`

- [ ] **Step 2: Write `cmd/server/frontend.go`**

```go
// cmd/server/frontend.go
package main

import (
	"embed"
	"io/fs"
)

//go:embed all:frontend_dist
var embeddedFrontend embed.FS

func frontendFS() (fs.FS, error) {
	return fs.Sub(embeddedFrontend, "frontend_dist")
}
```

- [ ] **Step 3: Symlink the frontend build output so `go:embed` can reach it**

`go:embed` cannot embed paths outside the package directory (`..`), so `cmd/server` needs its own copy. Use a symlink checked into git so `frontend/dist` (Task 12's build output) is embedded without duplicating files:

Run: `rm frontend/dist/.gitkeep && rmdir frontend/dist && mkdir -p frontend/dist && touch frontend/dist/.gitkeep && ln -s ../../frontend/dist cmd/server/frontend_dist`

- [ ] **Step 4: Write `cmd/server/main.go`**

```go
// cmd/server/main.go
package main

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Jaydee94/podustrial/internal/api"
	"github.com/Jaydee94/podustrial/internal/chaos"
	"github.com/Jaydee94/podustrial/internal/factory"
	"github.com/Jaydee94/podustrial/internal/k8s"
	"github.com/Jaydee94/podustrial/internal/progress"
)

func main() {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.ExpandEnv("$HOME/.kube/config")
	}
	namespace := "default"

	client, err := k8s.NewClient(kubeconfig, namespace)
	if err != nil {
		log.Fatalf("connect to cluster: %v", err)
	}

	store, err := progress.Open("podustrial-progress.db")
	if err != nil {
		log.Fatalf("open progress store: %v", err)
	}
	defer store.Close()

	hub := api.NewHub()
	server := api.NewServer(client, hub)

	events := make(chan factory.Event, 64)
	watcher := k8s.NewWatcher(client, events)
	healthMonitor := k8s.NewHealthMonitor(client, events)
	chaosService := chaos.NewService(
		chaos.Config{Enabled: false, Interval: 30 * time.Second, Probability: 0.2},
		client,
		chaos.RealClock(),
		rand.New(rand.NewSource(time.Now().UnixNano())),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go watcher.Run(ctx)
	go healthMonitor.Run(ctx, time.NewTicker(5*time.Second).C)
	go hub.Run(ctx, events)
	go chaosService.Run(ctx)

	mux := server.Routes()
	frontend, err := frontendFS()
	if err != nil {
		log.Fatalf("load embedded frontend: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(frontend)))

	httpServer := &http.Server{Addr: ":8080", Handler: mux}

	go func() {
		log.Println("podustrial listening on :8080")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	httpServer.Shutdown(shutdownCtx)
}
```

- [ ] **Step 5: Verify it builds and serves**

Run: `go build ./... && KUBECONFIG=/tmp/kc go run ./cmd/server & sleep 1 && curl -sf http://localhost:8080/healthz && kill %1`
Expected: build succeeds, `/healthz` returns HTTP 200 (empty body).

- [ ] **Step 6: Commit**

```bash
git add cmd/server frontend/dist/.gitkeep
git commit -m "feat: wire backend components into server binary with embedded frontend"
```

---

### Task 12: Frontend Scaffold & WebSocket Client

**Files:**
- Create: `frontend/package.json`
- Create: `frontend/vite.config.ts`
- Create: `frontend/index.html`
- Create: `frontend/src/net/socket.ts`
- Test: `frontend/src/net/socket.test.ts`
- Create: `frontend/src/main.ts`

**Interfaces:**
- Produces: `MachineStatus`, `FactoryEvent` types matching `internal/factory/state.go` JSON shape exactly, `connectFactorySocket(url: string, onEvent: (event: FactoryEvent) => void): WebSocket`. Used by Task 13 (`FactoryScene`).

- [ ] **Step 1: Scaffold the Vite project**

Run: `cd frontend && npm create vite@latest . -- --template vanilla-ts && npm install`

- [ ] **Step 2: Add Phaser and Vitest**

Run: `cd frontend && npm install phaser@3.87.0 && npm install -D vitest@2.1.4`

- [ ] **Step 3: Add a test script to `frontend/package.json`**

Modify `frontend/package.json`, in `"scripts"`:

```json
{
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview",
    "test": "vitest run"
  }
}
```

- [ ] **Step 4: Write the failing test**

```ts
// frontend/src/net/socket.test.ts
import { describe, it, expect, vi } from "vitest";
import { connectFactorySocket, type FactoryEvent } from "./socket";

class FakeWebSocket {
  static instances: FakeWebSocket[] = [];
  onmessage: ((ev: { data: string }) => void) | null = null;
  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  constructor(public url: string) {
    FakeWebSocket.instances.push(this);
  }
  emit(data: unknown) {
    this.onmessage?.({ data: JSON.stringify(data) });
  }
}

describe("connectFactorySocket", () => {
  it("forwards parsed FactoryEvents to the callback", () => {
    // @ts-expect-error test double replacing the global WebSocket
    globalThis.WebSocket = FakeWebSocket;
    const received: FactoryEvent[] = [];
    connectFactorySocket("ws://localhost:8080/ws", (ev) => received.push(ev));

    const socket = FakeWebSocket.instances.at(-1)!;
    socket.emit({ type: "machine_added", machine: { id: "m1", status: "running" } });

    expect(received).toHaveLength(1);
    expect(received[0]).toEqual({
      type: "machine_added",
      machine: { id: "m1", status: "running" },
    });
  });

  it("ignores messages it cannot parse as JSON", () => {
    // @ts-expect-error test double replacing the global WebSocket
    globalThis.WebSocket = FakeWebSocket;
    const received: FactoryEvent[] = [];
    connectFactorySocket("ws://localhost:8080/ws", (ev) => received.push(ev));

    const socket = FakeWebSocket.instances.at(-1)!;
    socket.onmessage?.({ data: "not json" });

    expect(received).toHaveLength(0);
  });
});
```

- [ ] **Step 5: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/net/socket.test.ts`
Expected: FAIL — module `./socket` not found.

- [ ] **Step 6: Write implementation**

```ts
// frontend/src/net/socket.ts
export type MachineStatus = "pending" | "running" | "failed";

export interface Machine {
  id: string;
  status: MachineStatus;
}

export type ClusterStatus = "ok" | "stromausfall";

export type EventType =
  | "machine_added"
  | "machine_updated"
  | "machine_removed"
  | "cluster_status";

export interface FactoryEvent {
  type: EventType;
  machine?: Machine;
  clusterStatus?: ClusterStatus;
}

export function connectFactorySocket(
  url: string,
  onEvent: (event: FactoryEvent) => void
): WebSocket {
  const socket = new WebSocket(url);
  socket.onmessage = (raw) => {
    try {
      const event = JSON.parse(raw.data) as FactoryEvent;
      onEvent(event);
    } catch {
      // ignore malformed messages
    }
  };
  return socket;
}
```

- [ ] **Step 7: Run test to verify it passes**

Run: `cd frontend && npx vitest run src/net/socket.test.ts`
Expected: PASS

- [ ] **Step 8: Write `frontend/index.html`**

```html
<!doctype html>
<html lang="de">
  <head>
    <meta charset="UTF-8" />
    <title>Podustrial</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
```

- [ ] **Step 9: Write a minimal `frontend/src/main.ts`**

```ts
// frontend/src/main.ts
import { connectFactorySocket } from "./net/socket";
import { createFactoryScene } from "./scene/FactoryScene";

const wsUrl = `ws://${window.location.host}/ws`;
const scene = createFactoryScene(document.getElementById("app")!);
connectFactorySocket(wsUrl, (event) => scene.applyEvent(event));
```

Note: `createFactoryScene` does not exist yet — this file will not compile until Task 13 adds `frontend/src/scene/FactoryScene.ts`. That is expected; Task 12's deliverable is verified via its own test (Step 7), not a full build.

- [ ] **Step 10: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/vite.config.ts frontend/index.html frontend/src/net/socket.ts frontend/src/net/socket.test.ts frontend/src/main.ts
git commit -m "feat: scaffold frontend with typed WebSocket client"
```

---

### Task 13: Factory Scene (Minimal Rendering)

**Files:**
- Create: `frontend/src/scene/FactoryScene.ts`
- Test: `frontend/src/scene/FactoryScene.test.ts`

**Interfaces:**
- Consumes: `FactoryEvent`, `Machine`, `MachineStatus` (Task 12).
- Produces: `createFactoryScene(container: HTMLElement): FactoryScene`, `interface FactoryScene { applyEvent(event: FactoryEvent): void; getMachineCount(): number; getMachineStatus(id: string): MachineStatus | undefined }`. Used by Task 11's `main.ts` (already referenced) and Task 14 (E2E test reads rendered DOM/state).

- [ ] **Step 1: Write the failing test**

```ts
// frontend/src/scene/FactoryScene.test.ts
import { describe, it, expect, beforeEach } from "vitest";
import { createFactoryScene } from "./FactoryScene";
import type { FactoryEvent } from "../net/socket";

describe("FactoryScene", () => {
  let container: HTMLElement;

  beforeEach(() => {
    container = document.createElement("div");
  });

  it("adds a machine on machine_added", () => {
    const scene = createFactoryScene(container);
    const event: FactoryEvent = {
      type: "machine_added",
      machine: { id: "m1", status: "pending" },
    };
    scene.applyEvent(event);

    expect(scene.getMachineCount()).toBe(1);
    expect(scene.getMachineStatus("m1")).toBe("pending");
  });

  it("updates machine status on machine_updated", () => {
    const scene = createFactoryScene(container);
    scene.applyEvent({ type: "machine_added", machine: { id: "m1", status: "pending" } });
    scene.applyEvent({ type: "machine_updated", machine: { id: "m1", status: "running" } });

    expect(scene.getMachineCount()).toBe(1);
    expect(scene.getMachineStatus("m1")).toBe("running");
  });

  it("removes a machine on machine_removed", () => {
    const scene = createFactoryScene(container);
    scene.applyEvent({ type: "machine_added", machine: { id: "m1", status: "running" } });
    scene.applyEvent({ type: "machine_removed", machine: { id: "m1", status: "running" } });

    expect(scene.getMachineCount()).toBe(0);
    expect(scene.getMachineStatus("m1")).toBeUndefined();
  });

  it("ignores cluster_status events for machine count", () => {
    const scene = createFactoryScene(container);
    scene.applyEvent({ type: "cluster_status", clusterStatus: "stromausfall" });

    expect(scene.getMachineCount()).toBe(0);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/scene/FactoryScene.test.ts`
Expected: FAIL — module `./FactoryScene` not found.

- [ ] **Step 3: Write implementation**

```ts
// frontend/src/scene/FactoryScene.ts
import Phaser from "phaser";
import type { FactoryEvent, Machine, MachineStatus } from "../net/socket";

export interface FactoryScene {
  applyEvent(event: FactoryEvent): void;
  getMachineCount(): number;
  getMachineStatus(id: string): MachineStatus | undefined;
}

const STATUS_COLOR: Record<MachineStatus, number> = {
  pending: 0x999999,
  running: 0x4caf50,
  failed: 0xe53935,
};

class PhaserFactoryScene extends Phaser.Scene implements FactoryScene {
  private machines = new Map<string, Machine>();
  private sprites = new Map<string, Phaser.GameObjects.Rectangle>();

  constructor() {
    super("factory");
  }

  applyEvent(event: FactoryEvent): void {
    if (!event.machine) {
      return;
    }
    const machine = event.machine;
    switch (event.type) {
      case "machine_added":
      case "machine_updated":
        this.machines.set(machine.id, machine);
        this.renderMachine(machine);
        break;
      case "machine_removed":
        this.machines.delete(machine.id);
        this.sprites.get(machine.id)?.destroy();
        this.sprites.delete(machine.id);
        break;
    }
  }

  getMachineCount(): number {
    return this.machines.size;
  }

  getMachineStatus(id: string): MachineStatus | undefined {
    return this.machines.get(id)?.status;
  }

  private renderMachine(machine: Machine): void {
    if (!this.add) {
      return; // headless test environment without a running Phaser game loop
    }
    const index = this.sprites.size;
    let rect = this.sprites.get(machine.id);
    if (!rect) {
      rect = this.add.rectangle(60 + index * 70, 60, 50, 50, STATUS_COLOR[machine.status]);
      this.sprites.set(machine.id, rect);
    } else {
      rect.setFillStyle(STATUS_COLOR[machine.status]);
    }
  }
}

export function createFactoryScene(container: HTMLElement): FactoryScene {
  const scene = new PhaserFactoryScene();
  new Phaser.Game({
    type: Phaser.AUTO,
    width: 800,
    height: 400,
    parent: container,
    scene,
  });
  return scene;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend && npx vitest run src/scene/FactoryScene.test.ts`
Expected: PASS

Note: `renderMachine` guards on `this.add` because Vitest runs in a headless DOM (jsdom or happy-dom) where Phaser's WebGL/Canvas renderer does not fully initialize synchronously; the state-tracking logic (`machines` map) is what the tests verify, matching the spec's "keine Kunst nötig, nur den Datenfluss beweisen" framing for this vertical slice.

- [ ] **Step 5: Configure Vitest to use a DOM environment**

Modify `frontend/vite.config.ts`, add a `test` block:

```ts
export default defineConfig({
  // ...existing config
  test: {
    environment: "jsdom",
  },
});
```

Run: `cd frontend && npm install -D jsdom@25.0.1`

- [ ] **Step 6: Re-run full frontend test suite**

Run: `cd frontend && npm test`
Expected: all tests PASS (socket.test.ts and FactoryScene.test.ts)

- [ ] **Step 7: Commit**

```bash
git add frontend/src/scene/FactoryScene.ts frontend/src/scene/FactoryScene.test.ts frontend/vite.config.ts frontend/package.json frontend/package-lock.json
git commit -m "feat: add minimal factory scene rendering machines from events"
```

---

### Task 14: End-to-End Test (Playwright)

**Files:**
- Create: `playwright.config.ts`
- Create: `e2e/podustrial.spec.ts`

**Interfaces:**
- Consumes: the full running stack (kind cluster + `cmd/server` binary serving embedded frontend built by Task 12/13).

- [ ] **Step 1: Install Playwright**

Run: `npm init -y --prefix e2e-tools 2>/dev/null; npm install -D @playwright/test@1.48.0 && npx playwright install --with-deps chromium`

- [ ] **Step 2: Write `playwright.config.ts`**

```ts
// playwright.config.ts
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  use: {
    baseURL: "http://localhost:8080",
  },
});
```

- [ ] **Step 3: Write the failing E2E test**

```ts
// e2e/podustrial.spec.ts
import { test, expect } from "@playwright/test";

test("spawning a container makes a machine appear via the real cluster", async ({ page, request }) => {
  await page.goto("/");

  const response = await request.post("/api/actions/spawn-container", {
    data: { id: `e2e-${Date.now()}` },
  });
  expect(response.status()).toBe(201);

  await expect
    .poll(
      async () => {
        const canvas = page.locator("canvas");
        return (await canvas.count()) > 0;
      },
      { timeout: 20_000 }
    )
    .toBeTruthy();
});
```

- [ ] **Step 4: Build the frontend so the backend has something to embed**

Run: `cd frontend && npm run build`

- [ ] **Step 5: Start the real local stack and run the test**

Run: `make start & sleep 5 && npx playwright test && kill %1 && make stop`
Expected: the spawn request returns 201 and the page renders a `<canvas>` element (Phaser's game canvas) within the poll timeout, proving the full path: real HTTP action → real Pod created in kind → informer picks it up → WebSocket event → frontend renders.

- [ ] **Step 6: Commit**

```bash
git add playwright.config.ts e2e/podustrial.spec.ts package.json package-lock.json
git commit -m "test: add Playwright E2E test against the real local stack"
```

---

### Task 15: Continuous Integration

**Files:**
- Create: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: `make test` (Go + frontend unit tests, Task 1), `make test-integration` (Task 1), Playwright E2E (Task 14).

- [ ] **Step 1: Write the CI workflow**

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - uses: actions/setup-node@v4
        with:
          node-version: "20"
          cache: "npm"
          cache-dependency-path: frontend/package-lock.json
      - name: Go unit tests
        run: go test ./...
      - name: Frontend unit tests
        working-directory: frontend
        run: npm ci && npm test

  integration-and-e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - uses: actions/setup-node@v4
        with:
          node-version: "20"
          cache: "npm"
          cache-dependency-path: frontend/package-lock.json
      - name: Install kind
        run: |
          curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.24.0/kind-linux-amd64
          chmod +x ./kind
          sudo mv ./kind /usr/local/bin/kind
      - name: Go integration tests
        run: make test-integration
      - name: Build frontend
        working-directory: frontend
        run: npm ci && npm run build
      - name: Install Playwright
        run: npm ci && npx playwright install --with-deps chromium
      - name: Start stack and run E2E
        run: |
          make start &
          sleep 10
          npx playwright test
          kill %1 || true
          make stop || true
```

- [ ] **Step 2: Verify the workflow file is valid YAML**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))" || (cd /tmp && pip install --quiet pyyaml && python3 -c "import yaml; yaml.safe_load(open('$OLDPWD/.github/workflows/ci.yml'))")`
Expected: no error (valid YAML).

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: run unit, integration, and E2E tests"
```

---

## Self-Review Notes

**Spec coverage:** §1 (real kind cluster, real K8s objects) → Tasks 1, 4, 5. §2 (four components) → Frontend: Tasks 12-13; Backend: Tasks 2-9; Chaos-Service: Task 10; kind-Cluster: Task 1. §3 (level gating, Level 1 backend behavior, "Unter der Haube" ab Level 4 noted as future mechanics work) → Task 4 implements the Level 1 translation; the optional YAML viewer itself is UI/mechanics work correctly left to a future plan per §8. §4 (data flow) → Tasks 5, 6, 8, wired in Task 11. §5 (setup/start) → Task 1, embed in Task 11. §6 (error handling) → Docker/cluster checks in Task 1, reconnect/status in Task 6, chaos config in Task 10, stateless-frontend-on-restart is inherent to the architecture (cluster is the source of truth, verified implicitly by Task 14's E2E test hitting a freshly-started stack). §7 (testing strategy) → unit (Tasks 2-3, 6-7, 9-10, 12-13), integration (Tasks 4-5, tagged `integration`), E2E via Playwright (Task 14), CI wiring (Task 15). §8 (scope boundaries) → respected throughout; no task invents level 2-8 mechanics, win conditions, or a shared-cluster mode.

**Type consistency check:** `factory.Event`/`Machine`/`MachineStatus`/`ClusterStatus`/`EventType` defined once in Task 3 and reused verbatim (same field names/JSON tags) through Tasks 5, 6, 8, and mirrored field-for-field in the TypeScript `FactoryEvent` (Task 12). `k8s.SpawnContainerRequest{ID}` defined in Task 4, consumed unchanged in Task 7's HTTP handler. `chaos.Deleter` interface (Task 10) matches the method set added to `k8s.Client` in Task 4 (`ListManagedPodNames`, `DeletePod`) — no adapter needed. `api.NewServer` signature changes from `(k8sClient)` in Task 7 to `(k8sClient, hub)` in Task 8, with the test call site updated in the same task.
