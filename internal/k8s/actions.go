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
