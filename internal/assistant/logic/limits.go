package logic

import (
	"context"
	"math"
	"sync"
	"time"
)

type RateLimitError struct{ RetryAfter int }

func (e *RateLimitError) Error() string { return "assistant rate limit exceeded" }

type bucket struct {
	count   int
	expires time.Time
}
type limiter struct {
	mu      sync.Mutex
	entries map[string]bucket
	limit   int
	window  time.Duration
	slots   chan struct{}
	now     func() time.Time
}

func newLimiter(limit int, window time.Duration, concurrent int) *limiter {
	if limit < 1 {
		limit = 10
	}
	if window <= 0 {
		window = time.Minute
	}
	if concurrent < 1 {
		concurrent = 4
	}
	return &limiter{entries: map[string]bucket{}, limit: limit, window: window, slots: make(chan struct{}, concurrent), now: time.Now}
}
func (l *limiter) acquire(ctx context.Context, key string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case l.slots <- struct{}{}:
	default:
		return nil, &RateLimitError{1}
	}
	release := func() { <-l.slots }
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for k, b := range l.entries {
		if !now.Before(b.expires) {
			delete(l.entries, k)
		}
	}
	b, exists := l.entries[key]
	if !exists && len(l.entries) >= 10000 {
		release()
		return nil, &RateLimitError{1}
	}
	if b.count >= l.limit {
		release()
		return nil, &RateLimitError{max(1, int(math.Ceil(b.expires.Sub(now).Seconds())))}
	}
	if !exists {
		b.expires = now.Add(l.window)
	}
	b.count++
	l.entries[key] = b
	return release, nil
}
