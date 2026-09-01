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
