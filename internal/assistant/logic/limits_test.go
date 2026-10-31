package logic

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestLimiterScopeConcurrencyAndExpiry(t *testing.T) {
	l := newLimiter(1, time.Minute, 1)
	now := time.Now()
	l.now = func() time.Time { return now }
	release, err := l.acquire(context.Background(), "company1:actor1")
	require.NoError(t, err)
	_, err = l.acquire(context.Background(), "company1:actor2")
	var rate *RateLimitError
	require.ErrorAs(t, err, &rate)
	assert.Equal(t, 1, rate.RetryAfter)
	release()
	_, err = l.acquire(context.Background(), "company1:actor1")
	require.ErrorAs(t, err, &rate)
	assert.Equal(t, 60, rate.RetryAfter)
	release, err = l.acquire(context.Background(), "company2:actor1")
	require.NoError(t, err)
	release()
	now = now.Add(time.Minute)
	release, err = l.acquire(context.Background(), "company1:actor1")
	require.NoError(t, err)
	release()
	assert.Len(t, l.entries, 1)
	for i := 0; i < 10000; i++ {
		l.entries[string(rune(i))+"x"] = bucket{1, now.Add(time.Minute)}
	}
	_, err = l.acquire(context.Background(), "new actor")
	require.ErrorAs(t, err, &rate)
	assert.Empty(t, l.slots)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = l.acquire(ctx, "x")
	assert.ErrorIs(t, err, context.Canceled)
}
