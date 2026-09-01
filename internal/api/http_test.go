package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/Jaydee94/podustrial/internal/k8s"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	client := k8s.NewFakeClient(fake.NewSimpleClientset(), "default")
	return NewServer(client, NewHub())
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
	if len(pod.Spec.Containers) != 1 || pod.Spec.Containers[0].Image != "busybox:1.36" {
		t.Errorf("unexpected containers: %+v", pod.Spec.Containers)
	}
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

func TestHandleSpawnAction_EmptyID_ReturnsBadRequest(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/actions/spawn-container", strings.NewReader(`{"id":""}`))
	rec := httptest.NewRecorder()

	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d, body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
