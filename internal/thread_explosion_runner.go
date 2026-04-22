package internal

import (
	"context"
	"log/slog"
	"sync"
)

type ThreadExplosionRunner struct {
	log                    *slog.Logger
	incrementErrorsCounter func()
}

func NewThreadExplosionRunner(log *slog.Logger, incrementErrorsCounter func()) *ThreadExplosionRunner {
	return &ThreadExplosionRunner{
		log,
		incrementErrorsCounter,
	}
}

func (r *ThreadExplosionRunner) Run(ctx context.Context, slowCall func(ctx context.Context) error) {
	var wg sync.WaitGroup

	// Launch 100 "greedy" goroutines
	// Each will invoke a blocking C-call, forcing the scheduler to spawn new system threads (M).
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := slowCall(ctx)

			if err != nil {
				r.log.Error("failed to execute limited action",
					"error", err,
				)
				r.incrementErrorsCounter()
			}
		}()
	}

	// Wait for all blocking calls to complete
	wg.Wait()
}
