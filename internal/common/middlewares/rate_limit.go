package middlewares

import (
	"github.com/gin-gonic/gin"
	"sea-codex-server/internal/common/dto"
	"strconv"
	"sync"
	"time"
)

type rateBucket struct {
	count   int
	expires time.Time
}
type RateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	buckets map[string]rateBucket
	now     func() time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{limit: limit, window: window, buckets: map[string]rateBucket{}, now: time.Now}
}
func (l *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		now := l.now()
		key := c.FullPath() + ":" + c.ClientIP()
		l.mu.Lock()
		for k, b := range l.buckets {
			if !now.Before(b.expires) {
				delete(l.buckets, k)
			}
		}
		b, ok := l.buckets[key]
		if !ok {
			b = rateBucket{expires: now.Add(l.window)}
		}
		allowed := b.count < l.limit && (ok || len(l.buckets) < 4096)
		if allowed {
			b.count++
			l.buckets[key] = b
		}
		l.mu.Unlock()
		if !allowed {
			seconds := int(b.expires.Sub(now).Seconds()) + 1
			if seconds < 1 {
				seconds = 1
			}
			c.Header("Retry-After", strconv.Itoa(seconds))
			dto.ErrorResponse(c, 429, "Too many requests")
			return
		}
		c.Next()
	}
}
