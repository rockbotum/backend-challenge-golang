package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Limit struct {
	Weight int
	Window time.Duration
}

func DepthWeight(depth int) int {
	switch {
	case depth <= 100:
		return 5
	case depth <= 500:
		return 25
	case depth <= 1000:
		return 50
	default:
		return 250
	}
}

type Clock func() time.Time

type Limiter struct {
	mu           sync.Mutex
	buckets      []bucket
	now          Clock
	blockedUntil time.Time
}

type bucket struct {
	limit  Limit
	tokens float64
	last   time.Time
}

func New(limits []Limit, clock Clock) *Limiter {
	if clock == nil {
		clock = time.Now
	}

	buckets := make([]bucket, 0, len(limits))

	for _, limit := range limits {
		if limit.Weight <= 0 || limit.Window <= 0 {
			continue
		}

		buckets = append(buckets, bucket{
			limit:  limit,
			tokens: float64(limit.Weight),
		})
	}

	return &Limiter{
		buckets: buckets,
		now:     clock,
	}
}

func (l *Limiter) Acquire(ctx context.Context, weight int) error {
	if weight <= 0 {
		weight = 1
	}

	for {
		wait := l.refill(weight)
		if wait <= 0 {
			return nil
		}

		timer := time.NewTimer(wait)

		select {
		case <-ctx.Done():
			timer.Stop()

			return fmt.Errorf("acquire %d request weight: %w", weight, ctx.Err())
		case <-timer.C:
		}
	}
}

func (l *Limiter) Backoff(d time.Duration) {
	if d <= 0 {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	until := now.Add(d)
	if until.After(l.blockedUntil) {
		l.blockedUntil = until
	}

	for i := range l.buckets {
		l.buckets[i].tokens = 0
		l.buckets[i].last = now
	}
}

func (l *Limiter) refill(weight int) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	var wait time.Duration

	if now.Before(l.blockedUntil) {
		wait = l.blockedUntil.Sub(now)
	}

	for i := range l.buckets {
		b := &l.buckets[i]
		b.refill(now)

		if b.tokens < float64(weight) {
			delay := b.windowFor(float64(weight) - b.tokens)
			if delay > wait {
				wait = delay
			}
		}
	}

	if wait > 0 {
		return wait
	}

	for i := range l.buckets {
		l.buckets[i].tokens -= float64(weight)
	}

	return 0
}

func (b *bucket) refill(now time.Time) {
	if b.last.IsZero() {
		b.last = now
		return
	}

	elapsed := now.Sub(b.last)
	if elapsed <= 0 {
		return
	}

	b.last = now
	b.tokens += elapsed.Seconds() * float64(b.limit.Weight) / b.limit.Window.Seconds()

	if b.tokens > float64(b.limit.Weight) {
		b.tokens = float64(b.limit.Weight)
	}
}

func (b *bucket) windowFor(missing float64) time.Duration {
	if missing <= 0 {
		return 0
	}

	perSecond := float64(b.limit.Weight) / b.limit.Window.Seconds()
	if perSecond <= 0 {
		return b.limit.Window
	}

	return time.Duration(missing / perSecond * float64(time.Second))
}
