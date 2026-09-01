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
