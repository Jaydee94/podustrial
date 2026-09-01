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
