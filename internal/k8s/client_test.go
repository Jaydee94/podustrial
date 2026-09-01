package k8s

import "testing"

func TestNewClient_InvalidKubeconfigPath_ReturnsError(t *testing.T) {
	_, err := NewClient("/nonexistent/kubeconfig", "default")
	if err == nil {
		t.Fatal("expected an error for a missing kubeconfig file, got nil")
	}
}
