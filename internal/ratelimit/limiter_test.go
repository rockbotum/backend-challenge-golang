package ratelimit

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestDepthWeight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		depth int
		want  int
	}{
		{name: "small", depth: 5, want: 5},
		{name: "boundary of first tier", depth: 100, want: 5},
		{name: "second tier", depth: 101, want: 25},
		{name: "boundary of second tier", depth: 500, want: 25},
		{name: "third tier", depth: 501, want: 50},
		{name: "fourth tier", depth: 1001, want: 250},
		{name: "largest tier", depth: 5000, want: 250},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := DepthWeight(test.depth); got != test.want {
				t.Errorf("DepthWeight(%d) = %d, want %d", test.depth, got, test.want)
			}
		})
	}
}

func TestLimiter_SkipsUnusableLimits(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		limiter := New([]Limit{
			{Weight: 0, Window: time.Minute},
			{Weight: 100, Window: 0},
			{Weight: 100, Window: time.Minute},
		}, nil)

		if err := limiter.Acquire(context.Background(), 100); err != nil {
			t.Fatalf("Acquire returned %v", err)
		}
	})
}

func TestLimiter_ExhaustedBudgetBlocksUntilRefill(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		limiter := New([]Limit{{Weight: 10, Window: 10 * time.Second}}, nil)

		if err := limiter.Acquire(context.Background(), 10); err != nil {
			t.Fatalf("Acquire returned %v", err)
		}

		acquired := make(chan error, 1)

		go func() {
			acquired <- limiter.Acquire(context.Background(), 10)
		}()

		synctest.Wait()

		select {
		case err := <-acquired:
			t.Fatalf("Acquire returned %v while the budget was empty", err)
		default:
		}

		time.Sleep(10 * time.Second)
		synctest.Wait()

		if err := <-acquired; err != nil {
			t.Fatalf("Acquire returned %v", err)
		}
	})
}

func TestLimiter_WaitsOnlyForTheMissingWeight(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		limiter := New([]Limit{{Weight: 10, Window: 10 * time.Second}}, nil)

		if err := limiter.Acquire(context.Background(), 10); err != nil {
			t.Fatalf("Acquire returned %v", err)
		}

		acquired := make(chan error, 1)

		go func() {
			acquired <- limiter.Acquire(context.Background(), 5)
		}()

		synctest.Wait()

		time.Sleep(4 * time.Second)
		synctest.Wait()

		select {
		case err := <-acquired:
			t.Fatalf("Acquire returned %v with only 4 of the 5 tokens refilled", err)
		default:
		}

		time.Sleep(time.Second)
		synctest.Wait()

		if err := <-acquired; err != nil {
			t.Fatalf("Acquire returned %v", err)
		}
	})
}

func TestLimiter_AcquireRespectsContext(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		limiter := New([]Limit{{Weight: 5, Window: time.Minute}}, nil)

		if err := limiter.Acquire(context.Background(), 5); err != nil {
			t.Fatalf("Acquire returned %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		if err := limiter.Acquire(ctx, 5); !errors.Is(err, context.Canceled) {
			t.Errorf("Acquire error = %v, want context.Canceled", err)
		}
	})
}

func TestLimiter_BackoffDrainsBudgetAndBlocks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		limiter := New([]Limit{{Weight: 100, Window: time.Minute}}, nil)

		limiter.Backoff(30 * time.Second)

		acquired := make(chan error, 1)

		go func() {
			acquired <- limiter.Acquire(context.Background(), 100)
		}()

		synctest.Wait()

		select {
		case err := <-acquired:
			t.Fatalf("Acquire returned %v during the backoff window", err)
		default:
		}

		time.Sleep(time.Minute)
		synctest.Wait()

		if err := <-acquired; err != nil {
			t.Fatalf("Acquire returned %v", err)
		}
	})
}

func TestLimiter_BackoffIgnoresNonPositiveDuration(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		limiter := New([]Limit{{Weight: 10, Window: time.Minute}}, nil)

		limiter.Backoff(0)
		limiter.Backoff(-time.Minute)

		if err := limiter.Acquire(context.Background(), 10); err != nil {
			t.Fatalf("Acquire returned %v", err)
		}
	})
}

func TestLimiter_AppliesEveryWindow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		limiter := New([]Limit{
			{Weight: 100, Window: time.Minute},
			{Weight: 10, Window: time.Second},
		}, nil)

		ctx := context.Background()

		if err := limiter.Acquire(ctx, 10); err != nil {
			t.Fatalf("Acquire returned %v", err)
		}

		time.Sleep(time.Second)

		if err := limiter.Acquire(ctx, 10); err != nil {
			t.Fatalf("second Acquire returned %v", err)
		}

		time.Sleep(9 * time.Second)

		exhausted, cancel := context.WithCancel(context.Background())
		cancel()

		err := limiter.Acquire(exhausted, 100)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Acquire error = %v, want context.Canceled while the minute window is empty", err)
		}
	})
}

func TestLimiter_NonPositiveWeightIsTreatedAsOne(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		limiter := New([]Limit{{Weight: 1, Window: time.Minute}}, nil)

		if err := limiter.Acquire(context.Background(), 0); err != nil {
			t.Fatalf("Acquire(0) returned %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := limiter.Acquire(ctx, 1)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Acquire error = %v, want context.Canceled once the single token is spent", err)
		}
	})
}

func TestLimiter_ConcurrentAcquireStaysWithinTheWindow(t *testing.T) {
	t.Parallel()

	const (
		workers   = 20
		perWorker = 3
		weight    = 10
	)

	limiter := New([]Limit{{Weight: workers * perWorker * weight, Window: time.Minute}}, nil)

	var (
		wg   sync.WaitGroup
		fail atomic.Bool
	)

	for range workers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range perWorker {
				if err := limiter.Acquire(context.Background(), weight); err != nil {
					fail.Store(true)

					return
				}
			}
		}()
	}

	wg.Wait()

	if fail.Load() {
		t.Fatal("at least one Acquire returned an error")
	}
}

func TestLimiter_ConcurrentAcquireSharesTheBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const (
			workers = 10
			weight  = 10
			budget  = 100
		)

		limiter := New([]Limit{{Weight: budget, Window: time.Minute}}, nil)

		granted := make(chan struct{}, workers)

		for range workers {
			go func() {
				if err := limiter.Acquire(context.Background(), weight); err != nil {
					return
				}

				granted <- struct{}{}
			}()
		}

		time.Sleep(time.Minute / 2)
		synctest.Wait()

		if got := len(granted); got != budget/weight {
			t.Errorf("granted %d acquisitions, want %d", got, budget/weight)
		}
	})
}
