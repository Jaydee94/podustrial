package chaos

import (
	"context"
	"log"
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
	if clock == nil {
		clock = RealClock()
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	if cfg.Interval <= 0 {
		// A non-positive interval would make clock.After fire immediately on
		// every loop iteration, hot-looping the service. Treat it the same
		// as disabled rather than trusting config-driven input blindly.
		cfg.Enabled = false
	}
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
			if err != nil {
				log.Printf("chaos: list managed pods: %v", err)
				continue
			}
			if len(names) == 0 {
				continue
			}
			target := names[s.rng.Intn(len(names))]
			if err := s.deleter.DeletePod(ctx, target); err != nil {
				log.Printf("chaos: delete pod %q: %v", target, err)
			}
		}
	}
}
