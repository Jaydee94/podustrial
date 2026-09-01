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
				select {
				case m.out <- factory.NewClusterStatusEvent(status):
					lastStatus = status
				case <-ctx.Done():
					return
				}
			}
		}
	}
}
